package testhelpers

import (
	"crypto/rand"
	"encoding/binary"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	mh "github.com/multiformats/go-multihash"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

// RandPeerID is a libp2p random peer ID generator.
// These peer.ID generators were copied from libp2p/go-testutil. We didn't bring in the
// whole repo as a dependency because we only need this small bit. However if we find
// ourselves using more and more pieces we should just take a dependency on it.
func RandPeerID() (peer.ID, error) {
	buf := make([]byte, 16)
	if n, err := rand.Read(buf); n != 16 || err != nil {
		if n != 16 && err == nil {
			err = errors.New("couldnt read 16 random bytes")
		}
		panic(err)
	}
	h, _ := mh.Sum(buf, mh.SHA2_256, -1)
	return peer.ID(h), nil
}

// RequireIntPeerID takes in an integer and creates a unique peer id for it.
func RequireIntPeerID(t *testing.T, i int64) peer.ID {
	buf := make([]byte, 16)
	n := binary.PutVarint(buf, i)
	h, err := mh.Sum(buf[:n], mh.IDENTITY, -1)
	require.NoError(t, err)
	pid, err := peer.IDFromBytes(h)
	require.NoError(t, err)
	return pid
}
