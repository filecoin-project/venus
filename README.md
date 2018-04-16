# Filecoin (go-filecoin)

[![codecov](https://codecov.io/gh/filecoin-project/go-filecoin/branch/master/graph/badge.svg?token=J5QWYWkgHT)](https://codecov.io/gh/filecoin-project/go-filecoin)
[![CircleCI](https://circleci.com/gh/filecoin-project/go-filecoin.svg?style=svg&circle-token=5a9d1cb48788b41d98bdfbc8b15298816ec71fea)](https://circleci.com/gh/filecoin-project/go-filecoin)

> Filecoin implementation in Go

Filecoin is a decentralized storage network that turns cloud storage into an algorithmic market. The
market runs on a blockchain with a native protocol token (also called "filecoin" or FIL), which miners earn
by providing storage to clients.

## Table of Contents

- [Development](#development)
  - [Install Go](#install-go)
  - [Clone](#clone)
  - [Install Dependencies](#install-dependencies)
  - [Testing](#testing)
  - [Supported Commands](#supported-commands)
- [Contribute](#contribute)

## Development

### Install Go

The build process for go-filecoin requires at least go version 1.10, which you can download [here][1].

(If you run into trouble, see the [Go install instructions][4]).

### Clone

```sh
> mkdir -p ${GOPATH}/src/github.com/filecoin-project
> git clone git@github.com:filecoin-project/go-filecoin.git ${GOPATH}/src/github.com/filecoin-project/go-filecoin
```

### Install Dependencies

go-filecoin's dependencies are managed by [gx][2]; this project is not "go gettable." To install gx, gometalinter, and
other build and test dependencies, run:

```sh
> cd ${GOPATH}/src/github.com/filecoin-project/go-filecoin
> go run ./build/*.go deps
```

### Testing

The filecoin binary must be built prior to testing changes made during development. To do so, run:

```sh
> go run ./build/*.go build
```

Then, run the tests:

```sh
> go run ./build/*.go test
```

Note: Build and test can be combined:

```sh
> go run ./build/*.go best
```

### Supported Commands

```sh
# Build
> go run ./build/*.go build

# Install
> go run ./build/*.go install

# Test
> go run ./build/*.go test

# Build & Test
> go run ./build/*.go best

# Coverage
> go run ./build/*.go test -cover

# Lint
> go run ./build/*.go lint

# Race
> go run ./build/*.go test -race

# Deps, Lint, Build, Test (with args passed to Test)
> go run ./build/*.go all
```

Note: Any flag passed to `go run ./build/*.go test` (e.g. `-cover`) will be passed on to `go test`.

## Contribute

See [the contribute file](CONTRIBUTING.md).

If editing the readme, please conform to the [standard-readme][3] specification.

[1]: https://golang.org/dl/
[2]: https://github.com/whyrusleeping/gx
[3]: https://github.com/RichardLitt/standard-readme
[4]: https://golang.org/doc/install
