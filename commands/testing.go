package commands

import (
	dstr "gx/ipfs/QmXRKBQA4wXP7xWbFiZsR1GP4HV6wMDQ1aWFxZZ4uBcPX9/go-datastore"
	bstr "gx/ipfs/QmaG4DZ4JaqEfvPWt5nPPgoTzhc1tr1T3f4Nu9Jpdm8ymY/go-ipfs-blockstore"
	hamt "gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"

	bsrv "github.com/ipfs/go-ipfs/blockservice"
	exch "github.com/ipfs/go-ipfs/exchange"
	offl "github.com/ipfs/go-ipfs/exchange/offline"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/node"
)

// TestCmdHarness is a struct whose fields are pointers to the various entities which can be used to create a Filecoin
// Node which is suitable for running command tests.
//
// TODO: Once the Node builder is in, we can eliminate this struct. See GitHub issue #225.
type TestCmdHarness struct {
	blockservice *bsrv.BlockService
	blockstore   *bstr.Blockstore
	chainmanager *core.ChainManager
	datastore    *dstr.MapDatastore
	exchange     *exch.Interface
	ilpdstore    *hamt.CborIpldStore
	node         *node.Node
}

// CreateInMemoryOfflineTestCmdHarness creates a Filecoin node in a configuration that is suitable for command testing.
// The returned Node's BlockService will write to an in-memory (MapDatastore) DataStore and will rely upon an offline
// Exchange.
//
// TODO: We will replace this function with a proper Node builder soon. See GitHub issue #225.
func CreateInMemoryOfflineTestCmdHarness() *TestCmdHarness {
	mdatastr := dstr.NewMapDatastore()
	blkstore := bstr.NewBlockstore(mdatastr)
	exchange := offl.Exchange(blkstore)
	blocksvc := bsrv.New(blkstore, exchange)
	ipldstor := &hamt.CborIpldStore{Blocks: blocksvc}
	chainmgr := core.NewChainManager(mdatastr, ipldstor)

	nd := &node.Node{
		Blockservice: blocksvc,
		ChainMgr:     chainmgr,
		CborStore:    ipldstor,
	}

	return &TestCmdHarness{
		blockservice: &blocksvc,
		blockstore:   &blkstore,
		chainmanager: chainmgr,
		datastore:    mdatastr,
		exchange:     &exchange,
		ilpdstore:    ipldstor,
		node:         nd,
	}
}
