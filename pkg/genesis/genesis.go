package genesis

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"golang.org/x/xerrors"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/go-merkledag"
	"github.com/ipld/go-car"
	"github.com/mitchellh/go-homedir"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/gen"
	genesis2 "github.com/filecoin-project/venus/pkg/gen/genesis"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/state"
)

var glog = logging.Logger("genesis")

// InitFunc is the signature for function that is used to create a genesis block.
type InitFunc func(cst cbor.IpldStore, bs blockstore.Blockstore) (*block.Block, error)

// Ticket is the ticket to place in the genesis block header (which can't be derived from a prior ticket),
// used in the evaluation of the messages in the genesis block,
// and *also* the ticket value used when computing the genesis state (the parent state of the genesis block).
var Ticket = block.Ticket{
	VRFProof: []byte{
		0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec,
		0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec, 0xec,
	},
}

// VM is the view into the VM used during genesis block creation.
type VM interface {
	ApplyGenesisMessage(from address.Address, to address.Address, method abi.MethodNum, value abi.TokenAmount, params interface{}) (*vm.Ret, error)
	Flush() (state.Root, error)
}

func MakeGenesis(ctx context.Context, rep repo.Repo, outFile, genesisTemplate string, para *config.ForkUpgradeConfig) InitFunc {
	return func(_ cbor.IpldStore, bs blockstore.Blockstore) (*block.Block, error) {
		glog.Warn("Generating new random genesis block, note that this SHOULD NOT happen unless you are setting up new network")
		genesisTemplate, err := homedir.Expand(genesisTemplate)
		if err != nil {
			return nil, err
		}

		fdata, err := ioutil.ReadFile(genesisTemplate)
		if err != nil {
			return nil, xerrors.Errorf("reading preseals json: %w", err)
		}

		var template genesis2.Template
		if err := json.Unmarshal(fdata, &template); err != nil {
			return nil, err
		}

		if template.Timestamp == 0 {
			template.Timestamp = uint64(constants.Clock.Now().Unix())
		}

		// TODO potentially replace this cached blockstore by a CBOR cache.
		cbs, err := blockstoreutil.CachedBlockstore(ctx, bs, blockstoreutil.DefaultCacheOpts())
		if err != nil {
			return nil, err
		}

		b, err := genesis2.MakeGenesisBlock(context.TODO(), rep, cbs, template, para)
		if err != nil {
			return nil, xerrors.Errorf("make genesis block: %w", err)
		}

		fmt.Printf("GENESIS MINER ADDRESS: t0%d\n", genesis2.MinerStart)

		f, err := os.OpenFile(outFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return nil, err
		}

		offl := offline.Exchange(cbs)
		blkserv := blockservice.New(cbs, offl)
		dserv := merkledag.NewDAGService(blkserv)

		if err := car.WriteCarWithWalker(context.TODO(), dserv, []cid.Cid{b.Genesis.Cid()}, f, gen.CarWalkFunc); err != nil {
			return nil, err
		}

		glog.Infof("WRITING GENESIS FILE AT %s", f.Name())

		if err := f.Close(); err != nil {
			return nil, err
		}

		return b.Genesis, nil

	}
}
