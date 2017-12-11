# go-filecoin

[![CircleCI](https://circleci.com/gh/filecoin-project/go-filecoin.svg?style=svg&circle-token=5a9d1cb48788b41d98bdfbc8b15298816ec71fea)](https://circleci.com/gh/filecoin-project/go-filecoin) [![Build status](https://ci.appveyor.com/api/projects/status/27ee44u7ets5je5m?svg=true)](https://ci.appveyor.com/project/FilecoinAdmin/go-filecoin)

> Filecoin Implementation in Go

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
