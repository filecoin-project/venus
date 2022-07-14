package messager

import (
	"context"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"

	shared "github.com/filecoin-project/venus/venus-shared/types"
	types "github.com/filecoin-project/venus/venus-shared/types/messager"
)

type IMessager interface {
	HasMessageByUid(ctx context.Context, id string) (bool, error)                                                                                              //perm:read
	WaitMessage(ctx context.Context, id string, confidence uint64) (*types.Message, error)                                                                     //perm:read
	ForcePushMessage(ctx context.Context, account string, msg *shared.Message, meta *types.SendSpec) (string, error)                                           //perm:admin
	ForcePushMessageWithId(ctx context.Context, id string, account string, msg *shared.Message, meta *types.SendSpec) (string, error)                          //perm:write
	PushMessage(ctx context.Context, msg *shared.Message, meta *types.SendSpec) (string, error)                                                                //perm:write
	PushMessageWithId(ctx context.Context, id string, msg *shared.Message, meta *types.SendSpec) (string, error)                                               //perm:write
	GetMessageByUid(ctx context.Context, id string) (*types.Message, error)                                                                                    //perm:read
	GetMessageBySignedCid(ctx context.Context, cid cid.Cid) (*types.Message, error)                                                                            //perm:read
	GetMessageByUnsignedCid(ctx context.Context, cid cid.Cid) (*types.Message, error)                                                                          //perm:read
	GetMessageByFromAndNonce(ctx context.Context, from address.Address, nonce uint64) (*types.Message, error)                                                  //perm:read
	ListMessage(ctx context.Context) ([]*types.Message, error)                                                                                                 //perm:admin
	ListMessageByFromState(ctx context.Context, from address.Address, state types.MessageState, isAsc bool, pageIndex, pageSize int) ([]*types.Message, error) //perm:admin
	ListMessageByAddress(ctx context.Context, addr address.Address) ([]*types.Message, error)                                                                  //perm:admin
	ListFailedMessage(ctx context.Context) ([]*types.Message, error)                                                                                           //perm:admin
	ListBlockedMessage(ctx context.Context, addr address.Address, d time.Duration) ([]*types.Message, error)                                                   //perm:admin
	UpdateMessageStateByID(ctx context.Context, id string, state types.MessageState) error                                                                     //perm:admin
	UpdateAllFilledMessage(ctx context.Context) (int, error)                                                                                                   //perm:admin
	UpdateFilledMessageByID(ctx context.Context, id string) (string, error)                                                                                    //perm:admin
	ReplaceMessage(ctx context.Context, params *types.ReplacMessageParams) (cid.Cid, error)                                                                    //perm:admin
	RepublishMessage(ctx context.Context, id string) error                                                                                                     //perm:admin
	MarkBadMessage(ctx context.Context, id string) error                                                                                                       //perm:admin
	RecoverFailedMsg(ctx context.Context, addr address.Address) ([]string, error)                                                                              //perm:admin

	GetAddress(ctx context.Context, addr address.Address) (*types.Address, error)                                                      //perm:admin
	HasAddress(ctx context.Context, addr address.Address) (bool, error)                                                                //perm:read
	WalletHas(ctx context.Context, addr address.Address) (bool, error)                                                                 //perm:read
	ListAddress(ctx context.Context) ([]*types.Address, error)                                                                         //perm:admin
	UpdateNonce(ctx context.Context, addr address.Address, nonce uint64) error                                                         //perm:admin
	DeleteAddress(ctx context.Context, addr address.Address) error                                                                     //perm:admin
	ForbiddenAddress(ctx context.Context, addr address.Address) error                                                                  //perm:admin
	ActiveAddress(ctx context.Context, addr address.Address) error                                                                     //perm:admin
	SetSelectMsgNum(ctx context.Context, addr address.Address, num uint64) error                                                       //perm:admin
	SetFeeParams(ctx context.Context, addr address.Address, gasOverEstimation, gasOverPremium float64, maxFee, maxFeeCap string) error //perm:admin
	ClearUnFillMessage(ctx context.Context, addr address.Address) (int, error)                                                         //perm:admin

	GetSharedParams(ctx context.Context) (*types.SharedSpec, error)      //perm:admin
	SetSharedParams(ctx context.Context, params *types.SharedSpec) error //perm:admin
	RefreshSharedParams(ctx context.Context) error                       //perm:admin

	SaveNode(ctx context.Context, node *types.Node) error          //perm:admin
	GetNode(ctx context.Context, name string) (*types.Node, error) //perm:admin
	HasNode(ctx context.Context, name string) (bool, error)        //perm:admin
	ListNode(ctx context.Context) ([]*types.Node, error)           //perm:admin
	DeleteNode(ctx context.Context, name string) error             //perm:admin

	SetLogLevel(ctx context.Context, level string) error //perm:admin

	Send(ctx context.Context, params types.QuickSendParams) (string, error) //perm:admin
}
