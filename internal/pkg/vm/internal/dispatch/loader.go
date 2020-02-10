package dispatch

import (
	"fmt"

	"github.com/ipfs/go-cid"
)

// CodeLoader allows yo to load an actor's code based on its id an epoch.
type CodeLoader struct {
	actors map[cid.Cid]Actor
}

// GetActorImpl returns executable code for an actor by code cid at a specific protocol version
func (cl CodeLoader) GetActorImpl(code cid.Cid) (Dispatcher, error) {
	actor, ok := cl.actors[code]
	if !ok {
		return nil, fmt.Errorf("Actor code not found. code: %s", code)
	}
	return &actorDispatcher{code: code, actor: actor}, nil
}

// CodeLoaderBuilder helps you build a CodeLoader.
type CodeLoaderBuilder struct {
	actors map[cid.Cid]Actor
}

// NewBuilder creates a builder to generate a builtin.Actor data structure
func NewBuilder() *CodeLoaderBuilder {
	return &CodeLoaderBuilder{actors: map[cid.Cid]Actor{}}
}

// Add lets you add an actor dispatch table for a given version.
func (b *CodeLoaderBuilder) Add(code cid.Cid, actor Actor) *CodeLoaderBuilder {
	b.actors[code] = actor
	return b
}

// Build builds the code loader.
func (b *CodeLoaderBuilder) Build() CodeLoader {
	return CodeLoader{actors: b.actors}
}
