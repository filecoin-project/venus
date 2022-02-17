package proof_event_api

import (
	"context"
	"github.com/filecoin-project/venus/venus-shared/types/gateway"
)

type IProofEventAPI interface {
	ResponseProofEvent(ctx context.Context, resp *gateway.ResponseEvent) error                                       //perm:write
	ListenProofEvent(ctx context.Context, policy *gateway.ProofRegisterPolicy) (<-chan *gateway.RequestEvent, error) //perm:write
}
