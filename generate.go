package slogx

//go:generate mise exec -- mockgen -destination=mock.handler_test.go -package=slogx_test log/slog Handler
