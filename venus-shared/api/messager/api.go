package messager

import (
	"context"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/api"
	"github.com/filecoin-project/venus/venus-shared/types"
	mtypes "github.com/filecoin-project/venus/venus-shared/types/messager"
)

type IMessager interface {
	HasMessageByUid(ctx context.Context, id string) (bool, error)                                                                                                //perm:read
	WaitMessage(ctx context.Context, id string, confidence uint64) (*mtypes.Message, error)                                                                      //perm:read
	ForcePushMessage(ctx context.Context, account string, msg *types.Message, meta *mtypes.SendSpec) (string, error)                                             //perm:admin
	ForcePushMessageWithId(ctx context.Context, id string, account string, msg *types.Message, meta *mtypes.SendSpec) (string, error)                            //perm:write
	PushMessage(ctx context.Context, msg *types.Message, meta *mtypes.SendSpec) (string, error)                                                                  //perm:write
	PushMessageWithId(ctx context.Context, id string, msg *types.Message, meta *mtypes.SendSpec) (string, error)                                                 //perm:write
	GetMessageByUid(ctx context.Context, id string) (*mtypes.Message, error)                                                                                     //perm:read
	GetMessageBySignedCid(ctx context.Context, cid cid.Cid) (*mtypes.Message, error)                                                                             //perm:read
	GetMessageByUnsignedCid(ctx context.Context, cid cid.Cid) (*mtypes.Message, error)                                                                           //perm:read
	GetMessageByFromAndNonce(ctx context.Context, from address.Address, nonce uint64) (*mtypes.Message, error)                                                   //perm:read
	ListMessage(ctx context.Context) ([]*mtypes.Message, error)                                                                                                  //perm:admin
	ListMessageByFromState(ctx context.Context, from address.Address, state mtypes.MessageState, isAsc bool, pageIndex, pageSize int) ([]*mtypes.Message, error) //perm:admin
	ListMessageByAddress(ctx context.Context, addr address.Address) ([]*mtypes.Message, error)                                                                   //perm:admin
	ListFailedMessage(ctx context.Context) ([]*mtypes.Message, error)                                                                                            //perm:admin
	ListBlockedMessage(ctx context.Context, addr address.Address, d time.Duration) ([]*mtypes.Message, error)                                                    //perm:admin
	UpdateMessageStateByID(ctx context.Context, id string, state mtypes.MessageState) error                                                                      //perm:admin
	UpdateAllFilledMessage(ctx context.Context) (int, error)                                                                                                     //perm:admin
	UpdateFilledMessageByID(ctx context.Context, id string) (string, error)                                                                                      //perm:admin
	ReplaceMessage(ctx context.Context, params *mtypes.ReplacMessageParams) (cid.Cid, error)                                                                     //perm:admin
	RepublishMessage(ctx context.Context, id string) error                                                                                                       //perm:admin
	MarkBadMessage(ctx context.Context, id string) error                                                                                                         //perm:admin
	RecoverFailedMsg(ctx context.Context, addr address.Address) ([]string, error)                                                                                //perm:admin

	GetAddress(ctx context.Context, addr address.Address) (*mtypes.Address, error)                                                     //perm:admin
	HasAddress(ctx context.Context, addr address.Address) (bool, error)                                                                //perm:read
	WalletHas(ctx context.Context, addr address.Address) (bool, error)                                                                 //perm:read
	ListAddress(ctx context.Context) ([]*mtypes.Address, error)                                                                        //perm:admin
	UpdateNonce(ctx context.Context, addr address.Address, nonce uint64) error                                                         //perm:admin
	DeleteAddress(ctx context.Context, addr address.Address) error                                                                     //perm:admin
	ForbiddenAddress(ctx context.Context, addr address.Address) error                                                                  //perm:admin
	ActiveAddress(ctx context.Context, addr address.Address) error                                                                     //perm:admin
	SetSelectMsgNum(ctx context.Context, addr address.Address, num uint64) error                                                       //perm:admin
	SetFeeParams(ctx context.Context, addr address.Address, gasOverEstimation, gasOverPremium float64, maxFee, maxFeeCap string) error //perm:admin
	ClearUnFillMessage(ctx context.Context, addr address.Address) (int, error)                                                         //perm:admin

	GetSharedParams(ctx context.Context) (*mtypes.SharedSpec, error)      //perm:admin
	SetSharedParams(ctx context.Context, params *mtypes.SharedSpec) error //perm:admin
	RefreshSharedParams(ctx context.Context) error                        //perm:admin

	SaveNode(ctx context.Context, node *mtypes.Node) error          //perm:admin
	GetNode(ctx context.Context, name string) (*mtypes.Node, error) //perm:admin
	HasNode(ctx context.Context, name string) (bool, error)         //perm:admin
	ListNode(ctx context.Context) ([]*mtypes.Node, error)           //perm:admin
	DeleteNode(ctx context.Context, name string) error              //perm:admin

	SetLogLevel(ctx context.Context, level string) error //perm:admin

	Send(ctx context.Context, params mtypes.QuickSendParams) (string, error) //perm:admin

	api.Version
}
