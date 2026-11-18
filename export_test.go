package slogx //nolint:testpackage // Exports internals for tests.

// Unexported sentinel errors of BootstrapHandlerConfig.Resolve, for tests.
var (
	ErrBootstrapInvalidMaxQueued = errBootstrapInvalidMaxQueued
)
