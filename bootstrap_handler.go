package slogx

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultBootstrapMaxQueued = 10000
	// A HandOffTo waiting for the handoff panics if it makes no progress for this long.
	bootstrapStuckTimeout = 10 * time.Second
)

// ErrBootstrapped is returned by a repeated call of [BootstrapHandler.HandOffTo].
var ErrBootstrapped = errors.New("slogx: BootstrapHandler: already handed off")

var errBootstrapInvalidMaxQueued = errors.New("slogx: BootstrapHandler: MaxQueued must not be negative")

// BootstrapHandlerConfig is a configuration for [NewBootstrapHandler].
//
// All fields are optional: the zero value means defaults.
type BootstrapHandlerConfig struct {
	// MaxQueued is the maximum number of records waiting for the handoff at a time.
	// A record logged when the queue is full is dropped,
	// as well as the following ones until the handoff takes the queue to replay it.
	// Records logged while the handoff replays the queue fill a new one,
	// so up to 2*MaxQueued records may be held at once.
	// Default is 10000.
	MaxQueued int
	// OnDrop, if not nil, is called for every dropped record by the goroutine which logs it.
	// It may count the record in a metric or panic.
	// It must not log through this handler or any handler derived from it.
	OnDrop func(ctx context.Context, r slog.Record)
	// DropReportLevel is the level of the record which reports the number of dropped records.
	// Default is [slog.LevelWarn].
	DropReportLevel slog.Leveler
	// NoDropReport turns off the report of dropped records.
	NoDropReport bool
}

// Resolve returns a ready-to-use deep copy of cfg:
// with defaults applied, values normalized, and everything validated.
// It is idempotent.
func (cfg BootstrapHandlerConfig) Resolve() (BootstrapHandlerConfig, error) {
	// Defaults.
	if cfg.MaxQueued == 0 {
		cfg.MaxQueued = defaultBootstrapMaxQueued
	}
	if cfg.DropReportLevel == nil {
		cfg.DropReportLevel = slog.LevelWarn
	}

	// Normalize.
	if cfg.NoDropReport {
		cfg.DropReportLevel = nil
	}

	// Validate.
	var err error
	if cfg.MaxQueued < 0 {
		err = errors.Join(err, errBootstrapInvalidMaxQueued)
	}
	return cfg, err
}

type bootstrapPhase int

const (
	bootstrapHolding bootstrapPhase = iota
	bootstrapHandingOff
	bootstrapPassthrough
)

// BootstrapHandler is an [slog.Handler] for the bootstrap of an application.
//
// It holds records of all levels in memory as is for a short time,
// until the application knows what, how and where to log
// (e.g. flags/envs/config are parsed, so the level and the type of the real handler are known).
// Handlers derived from BootstrapHandler (by WithAttrs and WithGroup) hold records too.
// Then [BootstrapHandler.HandOffTo] replays held records through the real handler,
// and BootstrapHandler together with all handlers derived from it becomes a pass-through:
// HandOffTo is called once, on any of them.
//
// An application which exits before HandOffTo prints nothing of the held records.
//
// A missing handoff is a bug: nothing is logged,
// at most [BootstrapHandlerConfig].MaxQueued records are held, the rest is dropped.
//
// BootstrapHandler keeps working as a pass-through after HandOffTo,
// so code which has captured it during the bootstrap (e.g. a logger stored by a library)
// doesn't lose records, at the cost of one extra method call (a few ns) per record.
// To avoid this cost replace it by the real handler using [BootstrapHandler.WithAttrs](nil).
//
// Hint: records are held as is, so [Stack] captures the stack of HandOffTo,
// use [ErrorStack] instead.
type BootstrapHandler struct {
	state *bootstrapState
	goa   *GroupOrAttrs // Own groups and attrs of this handler.
	// Not nil only in the pass-through phase:
	// it is the real handler with goa applied, for the fast path without a lock.
	passthrough atomic.Pointer[slog.Handler]
}

var _ slog.Handler = (*BootstrapHandler)(nil)

// NewBootstrapHandler returns a handler which holds records until HandOffTo.
func NewBootstrapHandler(cfg BootstrapHandlerConfig) (*BootstrapHandler, error) {
	cfg, err := cfg.Resolve()
	if err != nil {
		return nil, err
	}
	s := &bootstrapState{
		cfg:         cfg,
		phase:       bootstrapHolding,
		handOffDone: make(chan struct{}),
	}
	s.handOffOnce = sync.OnceFunc(s.handOff)
	h := &BootstrapHandler{state: s}
	s.nodes = []*BootstrapHandler{h}
	return h, nil
}

// MustBootstrapHandler is like [NewBootstrapHandler] but panics if cfg is invalid.
func MustBootstrapHandler(cfg BootstrapHandlerConfig) *BootstrapHandler {
	h, err := NewBootstrapHandler(cfg)
	if err != nil {
		panic(err)
	}
	return h
}

// Enabled implements [slog.Handler] interface.
func (h *BootstrapHandler) Enabled(ctx context.Context, l slog.Level) bool {
	next := h.passthrough.Load()
	if next != nil {
		return (*next).Enabled(ctx, l)
	}
	// Not a pass-through yet: the real handler is unknown, so everything is enabled.
	// The real handler filters held records when they are replayed.
	return true
}

// Handle implements [slog.Handler] interface.
func (h *BootstrapHandler) Handle(ctx context.Context, r slog.Record) error {
	next := h.passthrough.Load()
	if next == nil {
		if h.state.enqueue(&bootstrapRecord{ctx: ctx, goa: h.goa, r: r}) {
			return nil
		}
		next = h.passthrough.Load() // Lost a race with the end of the handoff.
	}
	if !(*next).Enabled(ctx, r.Level) { // See [BootstrapHandler.Enabled].
		return nil
	}
	return (*next).Handle(ctx, r)
}

// WithAttrs implements [slog.Handler] interface.
//
// After HandOffTo it returns a handler derived from the real handler,
// so h.WithAttrs(nil) returns the real handler with groups and attrs of h:
// use it to replace h by the real handler.
// Note: [slog.Logger.With] without args returns the same logger,
// so call WithAttrs(nil) on [slog.Logger.Handler].
func (h *BootstrapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := h.passthrough.Load()
	if next == nil {
		goa := h.goa.WithAttrs(attrs)
		if goa == h.goa {
			return h
		}
		n := h.derive(goa)
		if n != nil {
			return n
		}
		next = h.passthrough.Load() // Lost a race with the end of the handoff.
	}
	return (*next).WithAttrs(attrs)
}

// WithGroup implements [slog.Handler] interface.
//
// Unlike WithAttrs(nil), WithGroup("") returns h even after HandOffTo.
func (h *BootstrapHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := h.passthrough.Load()
	if next == nil {
		n := h.derive(h.goa.WithGroup(name))
		if n != nil {
			return n
		}
		next = h.passthrough.Load() // Lost a race with the end of the handoff.
	}
	return (*next).WithGroup(name)
}

// derive returns a handler with goa which waits for the handoff,
// or nil if the handoff is finished.
func (h *BootstrapHandler) derive(goa *GroupOrAttrs) *BootstrapHandler {
	s := h.state
	n := &BootstrapHandler{state: s, goa: goa}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase == bootstrapPassthrough {
		return nil
	}
	s.nodes = append(s.nodes, n)
	return n
}

// HandOffTo replays held records through next
// and makes all handlers derived from the same [NewBootstrapHandler]
// (not only h, also its parents and siblings) pass records straight to next.
//
// HandOffTo returns only when all held records are replayed and h is a pass-through,
// so records logged through next right after it keep the order.
// It may be called concurrently from several goroutines:
// the first call does the job, others wait for it and then return [ErrBootstrapped].
//
// It panics if next is nil, if next itself is derived from the same [NewBootstrapHandler],
// or when re-entering HandOffTo.
func (h *BootstrapHandler) HandOffTo(next slog.Handler) error {
	if next == nil {
		panic("slogx: nil Handler")
	}
	if next, ok := next.(*BootstrapHandler); ok && next.state == h.state {
		panic("slogx: BootstrapHandler: can't hand off to itself")
	}

	s := h.state
	s.mu.Lock()
	first := s.phase == bootstrapHolding
	if first {
		s.phase = bootstrapHandingOff
		s.handOffArg = next
	}
	s.mu.Unlock()

	if first {
		s.handOffOnce()
		return nil
	}
	s.handOffWatchdog()
	s.handOffOnce()
	return ErrBootstrapped
}

// bootstrapState holds the queue and the real handler.
// It is shared by a root BootstrapHandler and all handlers derived from it,
// which keep only their own groups and attrs.
type bootstrapState struct {
	cfg             BootstrapHandlerConfig
	handOffOnce     func()        // sync.OnceFunc(bootstrapState.handOff)
	handOffDone     chan struct{} // Closed when handOff ends, including by a panic.
	handOffProgress atomic.Uint64 // Counts foreign calls made by handOff.

	mu         sync.Mutex // Never held while foreign code runs.
	phase      bootstrapPhase
	handOffArg slog.Handler
	// Handlers (including the root) which wait for the handoff to derive the real handler.
	// Nothing is removed until then: a missing handoff is a bug, and a leak helps to notice it.
	nodes   []*BootstrapHandler
	queue   []bootstrapRecord
	dropped int // Dropped since the last queued; while >0 nothing is queued.
}

// bootstrapRecord is a held record: the arguments of Handle,
// where r doesn't include goa, which is applied at the replay by the real handler.
type bootstrapRecord struct {
	ctx context.Context //nolint:containedctx // It is a queued argument of Handle.
	goa *GroupOrAttrs
	r   slog.Record
}

// handOffWatchdog waits for the first HandOffTo to finish
// and panics if it makes no progress: foreign code called by handOff waits for this call.
func (s *bootstrapState) handOffWatchdog() {
	ticker := time.NewTicker(bootstrapStuckTimeout)
	defer ticker.Stop()
	last := s.handOffProgress.Load()
	for {
		select {
		case <-s.handOffDone:
			return
		case <-ticker.C:
			cur := s.handOffProgress.Load()
			if cur == last {
				panic("slogx: BootstrapHandler: handoff is stuck, possible deadlock on re-entering HandOffTo")
			}
			last = cur
		}
	}
}

// handOff derives the real handler for all handlers and replays the queue,
// until both are done, then switches all handlers to pass-through.
func (s *bootstrapState) handOff() {
	defer close(s.handOffDone)
	base := s.handOffArg

	// Each goa is applied to base separately, in full,
	// so the real handler may get more WithAttrs and WithGroup calls
	// than the application made on BootstrapHandler.
	derived := make(map[*GroupOrAttrs]slog.Handler)
	handler := func(goa *GroupOrAttrs) slog.Handler {
		h, ok := derived[goa]
		if !ok {
			h = goa.Handler(base)
			derived[goa] = h
		}
		return h
	}

	var allNodes []*BootstrapHandler
	var reportLevel slog.Level // Evaluated once: Level of a custom Leveler is foreign code.
	if !s.cfg.NoDropReport {
		reportLevel = s.cfg.DropReportLevel.Level()
	}

	for s.mu.Lock(); ; s.mu.Lock() {
		if len(s.nodes) == 0 && len(s.queue) == 0 {
			// Everything is derived, so this loop doesn't call foreign code.
			for _, n := range allNodes {
				h := handler(n.goa)
				n.passthrough.Store(&h)
			}
			s.phase = bootstrapPassthrough
			s.mu.Unlock()
			return
		}

		nodes, batch := s.nodes, s.queue
		s.nodes, s.queue = nil, nil
		if s.dropped > 0 && !s.cfg.NoDropReport {
			r := slog.NewRecord(time.Now(), reportLevel, "slogx: BootstrapHandler: dropped records", 0)
			r.AddAttrs(slog.Int("dropped", s.dropped))
			s.queue = []bootstrapRecord{{ctx: context.Background(), r: r}}
		}
		s.dropped = 0
		s.mu.Unlock()

		// Foreign code below: no lock!
		// Records and handlers which appear meanwhile land in s.queue and s.nodes,
		// and are processed by the next iteration.
		// So writers are never blocked by the handoff,
		// but a flow of records faster than next handles them
		// keeps the loop (HandOffTo) running.
		for _, n := range nodes {
			handler(n.goa)
			s.handOffProgress.Add(1)
		}
		allNodes = append(allNodes, nodes...)
		for i := range batch {
			br := &batch[i]
			next := handler(br.goa)
			if next.Enabled(br.ctx, br.r.Level) {
				_ = next.Handle(br.ctx, br.r)
			}
			s.handOffProgress.Add(1)
		}
	}
}

// enqueue holds e.
// It returns false if the handoff is finished, so e is not held.
func (s *bootstrapState) enqueue(e *bootstrapRecord) bool {
	s.mu.Lock()
	switch {
	case s.phase == bootstrapPassthrough:
		s.mu.Unlock()
		return false
	case s.dropped > 0 || len(s.queue) >= s.cfg.MaxQueued:
		s.dropped++
		s.mu.Unlock()
		if s.cfg.OnDrop != nil {
			s.cfg.OnDrop(e.ctx, e.r)
		}
		return true
	}
	s.queue = append(s.queue, *e)
	s.mu.Unlock()
	return true
}
