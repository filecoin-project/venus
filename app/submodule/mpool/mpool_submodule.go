package mpool

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/app/submodule/blockstore"
	chain2 "github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/mpool/msg"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/app/submodule/wallet"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/messagepool"
	"github.com/filecoin-project/venus/pkg/messagepool/journal"
	"github.com/filecoin-project/venus/pkg/net/msgsub"
	"github.com/filecoin-project/venus/pkg/net/pubsub"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm/state"
)

var log = logging.Logger("mpool")

type messagepoolConfig interface {
	Repo() repo.Repo
}

type chainReader interface {
	chain.TipSetProvider
	GetHead() block.TipSetKey
	GetTipSetReceiptsRoot(block.TipSetKey) (cid.Cid, error)
	GetTipSetStateRoot(block.TipSetKey) (cid.Cid, error)
	SubHeadChanges(context.Context) chan []*chain.HeadChange
	SubscribeHeadChanges(chain.ReorgNotifee)
}
type stateReader interface {
	ResolveAddressAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (address.Address, error)
	GetActorAt(ctx context.Context, tipKey block.TipSetKey, addr address.Address) (*types.Actor, error)
	GetTipSetState(context.Context, block.TipSetKey) (state.Tree, error)
}

// MessagingSubmodule enhances the `Node` with internal messaging capabilities.
type MessagePoolSubmodule struct { //nolint
	// Network Fields
	MessageTopic *pubsub.Topic
	MessageSub   pubsub.Subscription

	MPool     *messagepool.MessagePool
	chain     *chain2.ChainSubmodule
	walletAPI *wallet.WalletAPI
	Waiter    *msg.Waiter
}

func OpenFilesystemJournal(lr repo.Repo) (journal.Journal, error) {
	jrnl, err := journal.OpenFSJournal(lr, journal.EnvDisabledEvents())
	if err != nil {
		return nil, err
	}

	return jrnl, err
}

func NewMpoolSubmodule(cfg messagepoolConfig,
	network *network.NetworkSubmodule,
	chain *chain2.ChainSubmodule,
	syncer *syncer.SyncerSubmodule,
	bsModule *blockstore.BlockstoreSubmodule,
	wallet *wallet.WalletSubmodule,
) (*MessagePoolSubmodule, error) {
	mpp := messagepool.NewProvider(chain.ChainReader, chain.MessageStore, cfg.Repo().Config().NetworkParams, network.Pubsub)

	j, err := OpenFilesystemJournal(cfg.Repo())
	if err != nil {
		return nil, err
	}
	mp, err := messagepool.New(mpp, cfg.Repo().MetaDatastore(), network.NetworkName, syncer.Consensus, chain.State, j)
	if err != nil {
		return nil, xerrors.Errorf("constructing mpool: %s", err)
	}
	combineChainReader := struct {
		stateReader
		chainReader
	}{
		chain.State,
		chain.ChainReader,
	}
	waiter := msg.NewWaiter(combineChainReader, chain.MessageStore, bsModule.Blockstore, bsModule.CborStore)

	// setup messaging topic.
	// register block validation on pubsub
	msgSyntaxValidator := consensus.NewMessageSyntaxValidator()
	msgSignatureValidator := consensus.NewMessageSignatureValidator(chain.State)

	mtv := msgsub.NewMessageTopicValidator(msgSyntaxValidator, msgSignatureValidator)
	if err := network.Pubsub.RegisterTopicValidator(mtv.Topic(network.NetworkName), mtv.Validator(), mtv.Opts()...); err != nil {
		return nil, xerrors.Errorf("failed to register message validator: %s", err)
	}
	topic, err := network.Pubsub.Join(msgsub.Topic(network.NetworkName))
	if err != nil {
		return nil, err
	}

	return &MessagePoolSubmodule{
		MPool:        mp,
		MessageTopic: pubsub.NewTopic(topic),
		chain:        chain,
		walletAPI:    wallet.API(),
		Waiter:       waiter,
	}, nil
}

func (mp *MessagePoolSubmodule) Close() {
	err := mp.MPool.Close()
	if err != nil {
		log.Errorf("failed to close mpool: %s", err)
	}
}

func (mp *MessagePoolSubmodule) API() *MessagePoolAPI {
	return &MessagePoolAPI{mp: mp}
}
