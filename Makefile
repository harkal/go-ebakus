# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: ebakus android ios ebakus-cross evm all test clean
.PHONY: ebakus-linux ebakus-linux-386 ebakus-linux-amd64 ebakus-linux-mips64 ebakus-linux-mips64le
.PHONY: ebakus-linux-arm ebakus-linux-arm-5 ebakus-linux-arm-6 ebakus-linux-arm-7 ebakus-linux-arm64
.PHONY: ebakus-darwin ebakus-darwin-386 ebakus-darwin-amd64
.PHONY: ebakus-windows ebakus-windows-386 ebakus-windows-amd64

GOBIN = ./build/bin
GO ?= latest

ebakus:
	build/env.sh go run build/ci.go install ./cmd/ebakus
	@echo "Done building."
	@echo "Run \"$(GOBIN)/ebakus\" to launch ebakus."

all:
	build/env.sh go run build/ci.go install

android:
	build/env.sh go run build/ci.go aar --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/ebakus.aar\" to use the library."

ios:
	build/env.sh go run build/ci.go xcode --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/Ebakus.framework\" to use the library."

test: all
	build/env.sh go run build/ci.go test

lint: ## Run linters.
	build/env.sh go run build/ci.go lint

clean:
	./build/clean_go_build_cache.sh
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

devtools:
	env GOBIN= go get -u golang.org/x/tools/cmd/stringer
	env GOBIN= go get -u github.com/kevinburke/go-bindata/go-bindata
	env GOBIN= go get -u github.com/fjl/gencodec
	env GOBIN= go get -u github.com/golang/protobuf/protoc-gen-go
	env GOBIN= go install ./cmd/abigen
	@type "npm" 2> /dev/null || echo 'Please install node.js and npm'
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'

# Cross Compilation Targets (xgo)

ebakus-cross: ebakus-linux ebakus-darwin ebakus-windows ebakus-android ebakus-ios
	@echo "Full cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-*

ebakus-linux: ebakus-linux-386 ebakus-linux-amd64 ebakus-linux-arm ebakus-linux-mips64 ebakus-linux-mips64le
	@echo "Linux cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-linux-*

ebakus-linux-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/386 -v ./cmd/ebakus
	@echo "Linux 386 cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-linux-* | grep 386

ebakus-linux-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/amd64 -v ./cmd/ebakus
	@echo "Linux amd64 cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-linux-* | grep amd64

ebakus-linux-arm: ebakus-linux-arm-5 ebakus-linux-arm-6 ebakus-linux-arm-7 ebakus-linux-arm64
	@echo "Linux ARM cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-linux-* | grep arm

ebakus-linux-arm-5:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-5 -v ./cmd/ebakus
	@echo "Linux ARMv5 cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-linux-* | grep arm-5

ebakus-linux-arm-6:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-6 -v ./cmd/ebakus
	@echo "Linux ARMv6 cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-linux-* | grep arm-6

ebakus-linux-arm-7:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm-7 -v ./cmd/ebakus
	@echo "Linux ARMv7 cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-linux-* | grep arm-7

ebakus-linux-arm64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/arm64 -v ./cmd/ebakus
	@echo "Linux ARM64 cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-linux-* | grep arm64

ebakus-linux-mips:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips --ldflags '-extldflags "-static"' -v ./cmd/ebakus
	@echo "Linux MIPS cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-linux-* | grep mips

ebakus-linux-mipsle:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mipsle --ldflags '-extldflags "-static"' -v ./cmd/ebakus
	@echo "Linux MIPSle cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-linux-* | grep mipsle

ebakus-linux-mips64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips64 --ldflags '-extldflags "-static"' -v ./cmd/ebakus
	@echo "Linux MIPS64 cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-linux-* | grep mips64

ebakus-linux-mips64le:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=linux/mips64le --ldflags '-extldflags "-static"' -v ./cmd/ebakus
	@echo "Linux MIPS64le cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-linux-* | grep mips64le

ebakus-darwin: ebakus-darwin-386 ebakus-darwin-amd64
	@echo "Darwin cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-darwin-*

ebakus-darwin-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=darwin/386 -v ./cmd/ebakus
	@echo "Darwin 386 cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-darwin-* | grep 386

ebakus-darwin-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=darwin/amd64 -v ./cmd/ebakus
	@echo "Darwin amd64 cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-darwin-* | grep amd64

ebakus-windows: ebakus-windows-386 ebakus-windows-amd64
	@echo "Windows cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-windows-*

ebakus-windows-386:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=windows/386 -v ./cmd/ebakus
	@echo "Windows 386 cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-windows-* | grep 386

ebakus-windows-amd64:
	build/env.sh go run build/ci.go xgo -- --go=$(GO) --targets=windows/amd64 -v ./cmd/ebakus
	@echo "Windows amd64 cross compilation done:"
	@ls -ld $(GOBIN)/ebakus-windows-* | grep amd64
