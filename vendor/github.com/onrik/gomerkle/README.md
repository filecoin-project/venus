# Gomerkle
[![Build Status](https://travis-ci.org/onrik/gomerkle.svg?branch=master)](https://travis-ci.org/onrik/gomerkle)
[![Coverage Status](https://coveralls.io/repos/github/onrik/gomerkle/badge.svg?branch=master)](https://coveralls.io/github/onrik/gomerkle?branch=master)
[![GoDoc](https://godoc.org/github.com/onrik/gomerkle?status.svg)](https://godoc.org/github.com/onrik/gomerkle)
[![Go Report Card](https://goreportcard.com/badge/github.com/onrik/gomerkle)](https://goreportcard.com/report/github.com/onrik/gomerkle)

Golang Merkle tree implementation

## Usage
```go
package main

import (
    "crypto/sha256"

    "github.com/onrik/gomerkle"
)

func main() {
    data := [][]byte{
        []byte("Buzz"),
        []byte("Lenny"),
        []byte("Squeeze"),
        []byte("Wheezy"),
        []byte("Jessie"),
        []byte("Stretch"),
        []byte("Buster"),
    }
    tree := gomerkle.NewTree(sha256.New())
    tree.AddData(data...)

    err := tree.Generate()
    if err != nil {
        panic(err)
    }

    // Proof for Jessie
    proof := tree.GetProof(4)
    leaf := tree.GetLeaf(4)
    println(tree.VerifyProof(proof, tree.Root(), leaf))
}
```
