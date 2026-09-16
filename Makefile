.PHONY: fmt fmt-check vet test build check

# Use the host's null device even when GOOS selects a cross-compilation target.
NULL_DEVICE := $(if $(filter windows,$(shell go env GOHOSTOS)),NUL,/dev/null)

fmt:
	gofmt -w .

# GNU Make checks the file list without depending on POSIX shell conditionals.
# The gofmt command also fails for invalid Go syntax or unreadable source.
fmt-check:
	$(if $(strip $(shell gofmt -l .)),$(error Go source requires formatting; run make fmt))
	gofmt -d .

vet:
	go vet ./...

test:
	go test ./...

# Package verification only; no binary is written into the working tree.
build:
	go build -o $(NULL_DEVICE) ./...

check: fmt-check vet test build
