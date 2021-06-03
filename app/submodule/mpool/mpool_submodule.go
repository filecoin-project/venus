package mpool

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"time"

	"github.com/filecoin-project/venus/pkg/messagesigner"

	"github.com/filecoin-project/go-address"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/network"
	"github.com/filecoin-project/venus/app/submodule/syncer"
	"github.com/filecoin-project/venus/app/submodule/wallet"
	chainpkg "github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/messagepool"
	"github.com/filecoin-project/venus/pkg/messagepool/journal"
	"github.com/filecoin-project/venus/pkg/net/msgsub"
	"github.com/filecoin-project/venus/pkg/net/pubsub"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/types"
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

// MessagingSubmodule enhances the `Node` with internal messaging capabilities.
type MessagePoolSubmodule struct { //nolint
	// Network Fields
	MessageTopic *pubsub.Topic
	MessageSub   pubsub.Subscription

	MPool      *messagepool.MessagePool
	msgSigner  *messagesigner.MessageSigner
	chain      *chain.ChainSubmodule
	network    *network.NetworkSubmodule
	walletAPI  apiface.IWallet
	networkCfg *config.NetworkParamsConfig
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
	chain *chain.ChainSubmodule,
	syncer *syncer.SyncerSubmodule,
	wallet *wallet.WalletSubmodule,
	networkCfg *config.NetworkParamsConfig,
) (*MessagePoolSubmodule, error) {
	mpp := messagepool.NewProvider(chain.ChainReader, chain.MessageStore, cfg.Repo().Config().NetworkParams, network.Pubsub)

	j, err := OpenFilesystemJournal(cfg.Repo())
	if err != nil {
		return nil, err
	}
	mp, err := messagepool.New(mpp, cfg.Repo().MetaDatastore(), cfg.Repo().Config().NetworkParams.ForkUpgradeParam, cfg.Repo().Config().Mpool,
		network.NetworkName, syncer.Consensus, chain.ChainReader, j)
	if err != nil {
		return nil, xerrors.Errorf("constructing mpool: %s", err)
	}

	// setup messaging topic.
	// register block validation on pubsub
	msgSyntaxValidator := consensus.NewMessageSyntaxValidator()
	msgSignatureValidator := consensus.NewMessageSignatureValidator(chain.ChainReader)

	mtv := msgsub.NewMessageTopicValidator(msgSyntaxValidator, msgSignatureValidator)
	if err := network.Pubsub.RegisterTopicValidator(mtv.Topic(network.NetworkName), mtv.Validator(), mtv.Opts()...); err != nil {
		return nil, xerrors.Errorf("failed to register message validator: %s", err)
	}

	return &MessagePoolSubmodule{
		MPool:      mp,
		chain:      chain,
		walletAPI:  wallet.API(),
		network:    network,
		networkCfg: networkCfg,
		msgSigner:  messagesigner.NewMessageSigner(wallet.Wallet, mp, cfg.Repo().MetaDatastore()),
	}, nil
}

func (mp *MessagePoolSubmodule) handleIncomingMessage(ctx context.Context, pubSubMsg pubsub.Message) (err error) {
	sender := pubSubMsg.GetSender()

	// ignore messages from self
	if sender == mp.network.Host.ID() {
		return mp.validateLocalMessage(ctx, pubSubMsg)
	}

	unmarshaled := &types.SignedMessage{}
	if err := unmarshaled.UnmarshalCBOR(bytes.NewReader(pubSubMsg.GetData())); err != nil {
		return err
	}

	if err := mp.MPool.Add(ctx, unmarshaled); err != nil {
		log.Debugf("failed to add message from network to message pool (From: %s, To: %s, Nonce: %d, Value: %s): %s", unmarshaled.Message.From, unmarshaled.Message.To, unmarshaled.Message.Nonce, types.FIL(unmarshaled.Message.Value), err)
		switch {
		case xerrors.Is(err, messagepool.ErrSoftValidationFailure):
			fallthrough
		case xerrors.Is(err, messagepool.ErrRBFTooLowPremium):
			fallthrough
		case xerrors.Is(err, messagepool.ErrTooManyPendingMessages):
			fallthrough
		case xerrors.Is(err, messagepool.ErrNonceGap):
			fallthrough
		case xerrors.Is(err, messagepool.ErrNonceTooLow):
			return nil
		default:
			return err
		}
	}
	return err
}

func (mp *MessagePoolSubmodule) validateLocalMessage(ctx context.Context, msg pubsub.Message) error {
	m := &types.SignedMessage{}
	if err := m.UnmarshalCBOR(bytes.NewReader(msg.GetData())); err != nil {
		return err
	}

	if m.ChainLength() > messagepool.MaxMessageSize {
		log.Warnf("local message is too large! (%dB)", m.ChainLength())
		return xerrors.Errorf("local message is too large! (%dB)", m.ChainLength())
	}

	if m.Message.To == address.Undef {
		log.Warn("local message has invalid destination address")
		return xerrors.New("local message has invalid destination address")
	}

	if !m.Message.Value.LessThan(types.TotalFilecoinInt) {
		log.Warnf("local messages has too high value: %s", m.Message.Value)
		return xerrors.New("value-too-high")
	}

	if err := mp.MPool.VerifyMsgSig(m); err != nil {
		log.Warnf("signature verification failed for local message: %s", err)
		return xerrors.Errorf("verify-sig: %s", err)
	}

	return nil
}

// Start to the message pubsub topic to learn about messages to mine into blocks.
func (mp *MessagePoolSubmodule) Start(ctx context.Context) error {
	//setup topic
	topic, err := mp.network.Pubsub.Join(msgsub.Topic(mp.network.NetworkName))
	if err != nil {
		return err
	}
	subscribe := func() {
		mp.MessageTopic = pubsub.NewTopic(topic)
		mp.MessageSub, err = mp.MessageTopic.Subscribe()
		if err != nil {
			panic(errors.Wrapf(err, "failed to subscribe, topic: %s", topic))
		}

		go func() {
			for {
				received, err := mp.MessageSub.Next(ctx)
				if err != nil {
					if ctx.Err() != context.Canceled {
						log.Errorf("error reading message from topic %s: %s", mp.MessageTopic, err)
					}
					return
				}

				if err := mp.handleIncomingMessage(ctx, received); err != nil {
					handlerName := runtime.FuncForPC(reflect.ValueOf(mp.handleIncomingMessage).Pointer()).Name()
					if err != context.Canceled {
						log.Debugf("error in handler %s for topic %s: %s", handlerName, mp.MessageSub.Topic(), err)
					}
				}
			}
		}()
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

func (mp *MessagePoolSubmodule) API() apiface.IMessagePool {
	pushLocks := messagepool.NewMpoolLocker()
	return &MessagePoolAPI{mp: mp, pushLocks: pushLocks}
}

func (mp *MessagePoolSubmodule) V0API() apiface.IMessagePool {
	pushLocks := messagepool.NewMpoolLocker()
	return &MessagePoolAPI{mp: mp, pushLocks: pushLocks}
}
