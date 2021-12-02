package v0

import (
	"context"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/chain"
)

type ChainAPI interface {
	// MethodGroup: Chain
	// The Chain method group contains methods for interacting with the
	// blockchain, but that do not require any form of state computation.

	// ChainNotify returns channel with chain head updates.
	// First message is guaranteed to be of len == 1, and type == 'current'.
	ChainNotify(context.Context) (<-chan []*HeadChange, error) //perm:read

	// ChainHead returns the current head of the chain.
	ChainHead(context.Context) (*chain.TipSet, error) //perm:read

	// ChainGetRandomnessFromTickets is used to sample the chain for randomness.
	ChainGetRandomnessFromTickets(ctx context.Context, tsk chain.TipSetKey, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) //perm:read

	// ChainGetRandomnessFromBeacon is used to sample the beacon for randomness.
	ChainGetRandomnessFromBeacon(ctx context.Context, tsk chain.TipSetKey, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) //perm:read

	// ChainGetBlock returns the block specified by the given CID.
	ChainGetBlock(context.Context, cid.Cid) (*chain.BlockHeader, error) //perm:read
	// ChainGetTipSet returns the tipset specified by the given TipSetKey.
	ChainGetTipSet(context.Context, chain.TipSetKey) (*chain.TipSet, error) //perm:read

	// ChainGetBlockMessages returns messages stored in the specified block.
	//
	// Note: If there are multiple blocks in a tipset, it's likely that some
	// messages will be duplicated. It's also possible for blocks in a tipset to have
	// different messages from the same sender at the same nonce. When that happens,
	// only the first message (in a block with lowest ticket) will be considered
	// for execution
	//
	// NOTE: THIS METHOD SHOULD ONLY BE USED FOR GETTING MESSAGES IN A SPECIFIC BLOCK
	//
	// DO NOT USE THIS METHOD TO GET MESSAGES INCLUDED IN A TIPSET
	// Use ChainGetParentMessages, which will perform correct message deduplication
	ChainGetBlockMessages(ctx context.Context, blockCid cid.Cid) (*BlockMessages, error) //perm:read

	// ChainGetParentReceipts returns receipts for messages in parent tipset of
	// the specified block. The receipts in the list returned is one-to-one with the
	// messages returned by a call to ChainGetParentMessages with the same blockCid.
	ChainGetParentReceipts(ctx context.Context, blockCid cid.Cid) ([]*chain.MessageReceipt, error) //perm:read

	// ChainGetParentMessages returns messages stored in parent tipset of the
	// specified block.
	ChainGetParentMessages(ctx context.Context, blockCid cid.Cid) ([]Message, error) //perm:read

	// ChainGetMessagesInTipset returns message stores in current tipset
	ChainGetMessagesInTipset(ctx context.Context, tsk chain.TipSetKey) ([]Message, error) //perm:read

	// ChainGetTipSetByHeight looks back for a tipset at the specified epoch.
	// If there are no blocks at the specified epoch, a tipset at an earlier epoch
	// will be returned.
	ChainGetTipSetByHeight(context.Context, abi.ChainEpoch, chain.TipSetKey) (*chain.TipSet, error) //perm:read

	// ChainReadObj reads ipld nodes referenced by the specified CID from chain
	// blockstore and returns raw bytes.
	ChainReadObj(context.Context, cid.Cid) ([]byte, error) //perm:read

	// ChainDeleteObj deletes node referenced by the given CID
	ChainDeleteObj(context.Context, cid.Cid) error //perm:admin

	// ChainHasObj checks if a given CID exists in the chain blockstore.
	ChainHasObj(context.Context, cid.Cid) (bool, error) //perm:read

	// ChainStatObj returns statistics about the graph referenced by 'obj'.
	// If 'base' is also specified, then the returned stat will be a diff
	// between the two objects.
	ChainStatObj(ctx context.Context, obj cid.Cid, base cid.Cid) (ObjStat, error) //perm:read

	// ChainSetHead forcefully sets current chain head. Use with caution.
	ChainSetHead(context.Context, chain.TipSetKey) error //perm:admin

	// ChainGetGenesis returns the genesis tipset.
	ChainGetGenesis(context.Context) (*chain.TipSet, error) //perm:read

	// ChainTipSetWeight computes weight for the specified tipset.
	ChainTipSetWeight(context.Context, chain.TipSetKey) (chain.BigInt, error) //perm:read
	ChainGetNode(ctx context.Context, p string) (*IpldObject, error)          //perm:read

	// ChainGetMessage reads a message referenced by the specified CID from the
	// chain blockstore.
	ChainGetMessage(context.Context, cid.Cid) (*chain.Message, error) //perm:read

	// ChainGetPath returns a set of revert/apply operations needed to get from
	// one tipset to another, for example:
	//```
	//        to
	//         ^
	// from   tAA
	//   ^     ^
	// tBA    tAB
	//  ^---*--^
	//      ^
	//     tRR
	//```
	// Would return `[revert(tBA), apply(tAB), apply(tAA)]`
	ChainGetPath(ctx context.Context, from chain.TipSetKey, to chain.TipSetKey) ([]*HeadChange, error) //perm:read

	// ChainExport returns a stream of bytes with CAR dump of chain data.
	// The exported chain data includes the header chain from the given tipset
	// back to genesis, the entire genesis state, and the most recent 'nroots'
	// state trees.
	// If oldmsgskip is set, messages from before the requested roots are also not included.
	ChainExport(ctx context.Context, nroots abi.ChainEpoch, oldmsgskip bool, tsk chain.TipSetKey) (<-chan []byte, error) //perm:read
}
