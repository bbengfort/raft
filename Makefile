# Scripts to handle Raft build and installation
# Shell to use with Make
SHELL := /bin/bash

# Build Environment
PACKAGE = raft
PBPKG = $(CURDIR)/pb
BUILD = $(CURDIR)/_build
GIT_REVISION = $(shell git rev-parse --short HEAD)

# Commands
GOCMD = go
GODEP = dep ensure
GODOC = godoc
GINKGO = ginkgo
PROTOC = protoc
GORUN = $(GOCMD) run
GOGET = $(GOCMD) get
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean

# Output Helpers
BM  = $(shell printf "\033[34;1m●\033[0m")
GM = $(shell printf "\033[32;1m●\033[0m")
RM = $(shell printf "\033[31;1m●\033[0m")

# Export targets not associated with files.
.PHONY: all install build raft deps test citest clean doc protobuf compose

# Ensure dependencies are installed, run tests and compile
all: deps build test

# Install the commands and create configurations and data directories
install: build
	$(info $(GM) installing raft and making configuration …)
	@ cp $(BUILD)/raft /usr/local/bin/

# Build the various binaries and sources
build: protobuf raft

# Build the raft command and store in the build directory
raft:
	$(info $(GM) compiling raft executable on build $(GIT_REVISION) …)
	@ $(GOBUILD) -ldflags="-X 'github.com/bbengfort/raft.GitVersion=$(GIT_REVISION)'" -o $(BUILD)/raft ./cmd/raft

# Use dep to collect dependencies.
deps:
	$(info $(BM) fetching dependencies …)
	@ $(GODEP)

# Target for simple testing on the command line
test:
	$(info $(BM) running simple local tests …)
	@ $(GINKGO) -r

# Target for testing in continuous integration
citest:
	$(info $(BM) running CI tests with randomization and race …)
	$(GINKGO) -r -v --randomizeAllSpecs --randomizeSuites --failOnPending --cover --coverprofile=coverage.txt --covermode=atomic --trace --race --compilers=2

# Run Godoc server and open browser to the documentation
doc:
	$(info $(BM) running go documentation server at http://localhost:6060)
	$(info $(BM) type CTRL+C to exit the server)
	@ open http://localhost:6060/pkg/github.com/bbengfort/raft/
	@ $(GODOC) --http=:6060

# Clean build files
clean:
	$(info $(RM) cleaning up build …)
	@ $(GOCLEAN)
	@ find . -name "*.coverprofile" -print0 | xargs -0 rm -rf
	@ find . -name "coverage.txt" -print0 | xargs -0 rm -rf
	@ rm -rf $(BUILD)

# Compile protocol buffers
protobuf:
	$(info $(GM) compiling protocol buffers …)
	@ go generate ./...

# Run docker compose
compose:
	$(info $(GM) building docker compose images)
	@ docker compose -p raft build
	@ docker compose -p raft up