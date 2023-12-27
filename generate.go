package slogx

//go:generate -command MOCKGEN sh -c "$(git rev-parse --show-toplevel)/.buildcache/bin/$DOLLAR{DOLLAR}0 \"$DOLLAR{DOLLAR}@\"" mockgen
//go:generate MOCKGEN -destination=mock.handler_test.go -package=slogx_test log/slog Handler
