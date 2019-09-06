export SHELL := /bin/bash
export COMMIT := $(shell git log -n 1 --format=%H)

export GO111MODULE=on
export GOCMD=go
export GOBUILD=$(GOCMD) build
export GOTEST=$(GOCMD) test
export GOCLEAN=$(GOCMD) clean

