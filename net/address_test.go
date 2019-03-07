package net

import (
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestPeerAddrsToPeerInfosSuccess(t *testing.T) {
	assert := assert.New(t)

	addrs := []string{
		"/ip4/127.0.0.1/ipfs/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	}
	pis, err := PeerAddrsToPeerInfos(addrs)
	assert.NoError(err)
	assert.Equal("QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt", pis[0].ID.Pretty())
	assert.Equal("QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ", pis[1].ID.Pretty())
}

func TestPeerAddrsToPeerInfosFailure(t *testing.T) {
	assert := assert.New(t)

	addrs := []string{
		"/ipv4/no/such/address/ipfs/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
	}
	_, err := PeerAddrsToPeerInfos(addrs)
	assert.Error(err)
}
