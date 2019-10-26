package net

import (
	"testing"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestPeerAddrsToPeerInfosSuccess(t *testing.T) {
	tf.UnitTest(t)

	addrs := []string{
		"/ip4/127.0.0.1/ipfs/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	}
	pis, err := PeerAddrsToAddrInfo(addrs)
	assert.NoError(t, err)
	assert.Equal(t, "QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt", pis[0].ID.Pretty())
	assert.Equal(t, "QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ", pis[1].ID.Pretty())
}

func TestPeerAddrsToPeerInfosFailure(t *testing.T) {
	tf.UnitTest(t)

	addrs := []string{
		"/ipv4/no/such/address/ipfs/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
	}
	_, err := PeerAddrsToAddrInfo(addrs)
	assert.Error(t, err)
}
