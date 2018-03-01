# Filecoin (go-filecoin)

[![codecov](https://codecov.io/gh/filecoin-project/go-filecoin/branch/master/graph/badge.svg?token=J5QWYWkgHT)](https://codecov.io/gh/filecoin-project/go-filecoin)
[![CircleCI](https://circleci.com/gh/filecoin-project/go-filecoin.svg?style=svg&circle-token=5a9d1cb48788b41d98bdfbc8b15298816ec71fea)](https://circleci.com/gh/filecoin-project/go-filecoin)

Filecoin is a decentralized storage network that turns cloud storage into an algorithmic market. The
market runs on a blockchain with a native protocol token (also called “Filecoin”), which miners earn
by providing storage to clients.

## Development

```sh
# Install dependencies
> go run ./build/*.go deps
# Lint
> go run ./build/*.go lint
# Build
> go run ./build/*.go build
# Test
> go run ./build/*.go test
# Coverage
> go run ./build/*.go test -cover
# Race
> go run ./build/*.go test -race
```
