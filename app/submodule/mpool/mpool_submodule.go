package mpool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/filecoin-project/go-address"
	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/app/submodule/wallet"
	chainpkg "github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/messagepool"
	"github.com/filecoin-project/venus/pkg/messagepool/journal"
	"github.com/filecoin-project/venus/pkg/net/msgsub"
	"github.com/filecoin-project/venus/pkg/repo"
	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var pubsubMsgsSyncEpochs = 10

func init() {
	if s := os.Getenv("VENUS_MSGS_SYNC_EPOCHS"); s != "" {
		val, err := strconv.Atoi(s)
		if err != nil {
			log.Errorf("failed to parse LOTUS_MSGS_SYNC_EPOCHS: %s", err)
			return
		}
		pubsubMsgsSyncEpochs = val
	}
}

var log = logging.Logger("mpool")

type messagepoolConfig interface {
	Repo() repo.Repo
}

// MessagingSubmodule enhances the `Node` with internal message capabilities.
type MessagePoolSubmodule struct { //nolint
	// Network Fields
	MessageTopic *pubsub.Topic
	MessageSub   *pubsub.Subscription

	MPool      *messagepool.MessagePool
	msgSigner  *messagepool.MessageSigner
	chain      *chain.ChainSubmodule
	network    *network.NetworkSubmodule
	walletAPI  v1api.IWallet
	networkCfg *config.NetworkParamsConfig
}

func OpenFilesystemJournal(lr repo.Repo) (journal.Journal, error) {
	jrnl, err := journal.OpenFSJournal(lr, journal.EnvDisabledEvents())
	if err != nil {
		return nil, err
	}

	return jrnl, err
}

func NewMpoolSubmodule(ctx context.Context, cfg messagepoolConfig,
	network *network.NetworkSubmodule,
	chain *chain.ChainSubmodule,
	wallet *wallet.WalletSubmodule,
) (*MessagePoolSubmodule, error) {
	mpp := messagepool.NewProvider(chain.Stmgr, chain.ChainReader, chain.MessageStore, cfg.Repo().Config().NetworkParams, network.Pubsub)

	j, err := OpenFilesystemJournal(cfg.Repo())
	if err != nil {
		return nil, err
	}
	networkParams := cfg.Repo().Config().NetworkParams
	mp, err := messagepool.New(ctx, mpp, chain.Stmgr, cfg.Repo().MetaDatastore(), networkParams.ForkUpgradeParam,
		cfg.Repo().Config().Mpool, network.NetworkName, j)
	if err != nil {
		return nil, fmt.Errorf("constructing mpool: %s", err)
	}

	return &MessagePoolSubmodule{
		MPool:      mp,
		chain:      chain,
		walletAPI:  wallet.API(),
		network:    network,
		networkCfg: cfg.Repo().Config().NetworkParams,
		msgSigner:  messagepool.NewMessageSigner(wallet.WalletIntersection(), mp, cfg.Repo().MetaDatastore()),
	}, nil
}

func (mp *MessagePoolSubmodule) handleIncomingMessage(ctx context.Context) {
	for {
		_, err := mp.MessageSub.Next(ctx)
		if err != nil {
			log.Warn("error from message subscription: ", err)
			if ctx.Err() != nil {
				log.Warn("quitting HandleIncomingMessages loop")
				return
			}
			continue
		}
	}
}

func (mp *MessagePoolSubmodule) Validate(ctx context.Context, pid peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	if pid == mp.network.Host.ID() {
		return mp.validateLocalMessage(ctx, msg)
	}

	m := &types.SignedMessage{}
	if err := m.UnmarshalCBOR(bytes.NewReader(msg.GetData())); err != nil {
		log.Warnf("failed to decode incoming message: %s", err)
		return pubsub.ValidationReject
	}

	log.Debugf("validate incoming msg:%s", m.Cid().String())

	if err := mp.MPool.Add(ctx, m); err != nil {
		log.Debugf("failed to add message from network to message pool (From: %s, To: %s, Nonce: %d, Value: %s): %s", m.Message.From, m.Message.To, m.Message.Nonce, types.FIL(m.Message.Value), err)
		switch {
		case errors.Is(err, messagepool.ErrSoftValidationFailure):
			fallthrough
		case errors.Is(err, messagepool.ErrRBFTooLowPremium):
			fallthrough
		case errors.Is(err, messagepool.ErrTooManyPendingMessages):
			fallthrough
		case errors.Is(err, messagepool.ErrNonceGap):
			fallthrough
		case errors.Is(err, messagepool.ErrNonceTooLow):
			return pubsub.ValidationIgnore
		default:
			return pubsub.ValidationReject
		}
	}
	return pubsub.ValidationAccept
}

func (mp *MessagePoolSubmodule) validateLocalMessage(ctx context.Context, msg *pubsub.Message) pubsub.ValidationResult {
	m := &types.SignedMessage{}
	if err := m.UnmarshalCBOR(bytes.NewReader(msg.GetData())); err != nil {
		return pubsub.ValidationIgnore
	}

	if m.ChainLength() > messagepool.MaxMessageSize {
		log.Warnf("local message is too large! (%dB)", m.ChainLength())
		return pubsub.ValidationIgnore
	}

	if m.Message.To == address.Undef {
		log.Warn("local message has invalid destination address")
		return pubsub.ValidationIgnore
	}

	if !m.Message.Value.LessThan(types.TotalFilecoinInt) {
		log.Warnf("local messages has too high value: %s", m.Message.Value)
		return pubsub.ValidationIgnore
	}

	if err := mp.MPool.VerifyMsgSig(m); err != nil {
		log.Warnf("signature verification failed for local message: %s", err)
		return pubsub.ValidationIgnore
	}

	return pubsub.ValidationAccept
}

// Start to the message pubsub topic to learn about messages to mine into blocks.
func (mp *MessagePoolSubmodule) Start(ctx context.Context) error {
	topicName := msgsub.Topic(mp.network.NetworkName)
	var err error
	if err = mp.network.Pubsub.RegisterTopicValidator(topicName, mp.Validate); err != nil {
		return err
	}

	subscribe := func() {
		var err error
		if mp.MessageTopic, err = mp.network.Pubsub.Join(topicName); err != nil {
			panic(err)
		}
		if mp.MessageSub, err = mp.MessageTopic.Subscribe(); err != nil {
			panic(err)
		}
		go mp.handleIncomingMessage(ctx)
	}

	// wait until we are synced within 10 epochs
	go mp.waitForSync(pubsubMsgsSyncEpochs, subscribe)

	return nil
}

func (mp *MessagePoolSubmodule) waitForSync(epochs int, subscribe func()) {
	nearsync := time.Duration(epochs*int(mp.networkCfg.BlockDelay)) * time.Second

	// early check, are we synced at start up?
	ts := mp.chain.ChainReader.GetHead()
	timestamp := ts.MinTimestamp()
	timestampTime := time.Unix(int64(timestamp), 0)
	if constants.Clock.Since(timestampTime) < nearsync {
		subscribe()
		return
	}

	// we are not synced, subscribe to head changes and wait for sync
	mp.chain.ChainReader.SubscribeHeadChanges(func(rev, app []*types.TipSet) error {
		if len(app) == 0 {
			return nil
		}

		latest := app[0].MinTimestamp()
		for _, ts := range app[1:] {
			timestamp := ts.MinTimestamp()
			if timestamp > latest {
				latest = timestamp
			}
		}

		latestTime := time.Unix(int64(latest), 0)
		if constants.Clock.Since(latestTime) < nearsync {
			subscribe()
			return chainpkg.ErrNotifeeDone
		}

		return nil
	})
}

func (mp *MessagePoolSubmodule) Stop(ctx context.Context) {
	err := mp.MPool.Close()
	if err != nil {
		log.Errorf("failed to close mpool: %s", err)
	}
	if mp.MessageSub != nil {
		mp.MessageSub.Cancel()
	}
}

//API create a new mpool api implement
func (mp *MessagePoolSubmodule) API() v1api.IMessagePool {
	pushLocks := messagepool.NewMpoolLocker()
	return &MessagePoolAPI{mp: mp, pushLocks: pushLocks}
}

func (mp *MessagePoolSubmodule) V0API() v0api.IMessagePool {
	pushLocks := messagepool.NewMpoolLocker()
	return &MessagePoolAPI{mp: mp, pushLocks: pushLocks}
}
