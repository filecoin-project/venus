package v1

import (
	"context"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-f3/certs"
	"github.com/filecoin-project/go-f3/gpbft"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type IF3 interface {
	//*********************************** ALL F3 APIs below are not stable & subject to change ***********************************

	// F3Participate should be called by a storage provider to participate in signing F3 consensus.
	// Calling this API gives the lotus node a lease to sign in F3 on behalf of given SP.
	// The lease should be active only on one node. The lease will expire at the newLeaseExpiration.
	// To continue participating in F3 with the given node, call F3Participate again before
	// the newLeaseExpiration time.
	// newLeaseExpiration cannot be further than 5 minutes in the future.
	// It is recommended to call F3Participate every 60 seconds
	// with newLeaseExpiration set 2min into the future.
	// The oldLeaseExpiration has to be set to newLeaseExpiration of the last successful call.
	// For the first call to F3Participate, set the oldLeaseExpiration to zero value/time in the past.
	// F3Participate will return true if the lease was accepted.
	// The minerID has to be the ID address of the miner.
	F3Participate(ctx context.Context, minerID address.Address, newLeaseExpiration time.Time, oldLeaseExpiration time.Time) (bool, error) //perm:sign
	// F3GetCertificate returns a finality certificate at given instance number
	F3GetCertificate(ctx context.Context, instance uint64) (*certs.FinalityCertificate, error) //perm:read
	// F3GetLatestCertificate returns the latest finality certificate
	F3GetLatestCertificate(ctx context.Context) (*certs.FinalityCertificate, error) //perm:read
	// F3GetECPowerTable returns a F3 specific power table for use in standalone F3 nodes.
	F3GetECPowerTable(ctx context.Context, tsk types.TipSetKey) (gpbft.PowerEntries, error) //perm:read
	// F3GetF3PowerTable returns a F3 specific power table.
	F3GetF3PowerTable(ctx context.Context, tsk types.TipSetKey) (gpbft.PowerEntries, error) //perm:read
}
