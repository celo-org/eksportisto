# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: evm all test clean

GO ?= latest
CELO_BLOCKCHAIN_PATH?=../celo-blockchain
CELO_MONOREPO_PATH?=../celo-monorepo
GITHUB_ORG?=celo-org
GITHUB_REPO?=monitor

LSB_exists := $(shell command -v lsb_release 2> /dev/null)
GOLANGCI_exists := $(shell command -v golangci-lint 2> /dev/null)

COMMIT_SHA=$(shell git rev-parse HEAD)

OS :=
ifeq ("$(LSB_exists)","")
	OS = darwin
else
	OS = linux
endif

all: build

build:
	go build ./...

test: 
	go test ./...

lint: ## Run linters.
ifeq ("$(GOLANGCI_exists)","")
	$(error "No golangci in PATH, consult https://github.com/golangci/golangci-lint#install")
else
	golangci-lint run -c .golangci.yml
endif

clean-geth:
	go clean -cache
	
clean: clean-geth

