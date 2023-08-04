//go:build tools

//go:generate mkdir -p .buildcache/bin
//go:generate -command GOINSTALL env "GOBIN=$PWD/.buildcache/bin" go install

package tools

//go:generate GOINSTALL github.com/golang/mock/mockgen@v1.6.0
//go:generate GOINSTALL github.com/golangci/golangci-lint/cmd/golangci-lint@v1.52.0
//go:generate GOINSTALL github.com/mattn/goveralls@v0.0.12
