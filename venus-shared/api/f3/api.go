package f3

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-f3/certs"
)

type F3 interface {
	// F3Participate should be called by a miner node to participate in signing F3 consensus.
	// The address should be of type ID
	// The returned channel will never be closed by the F3
	// If it is closed without the context being cancelled, the caller should retry.
	// The values returned on the channel will inform the caller about participation
	// Empty strings will be sent if participation succeeded, non-empty strings explain possible errors.
	F3Participate(ctx context.Context, minerID address.Address) (<-chan string, error) //perm:admin
	// F3GetCertificate returns a finality certificate at given instance number
	F3GetCertificate(ctx context.Context, instance uint64) (*certs.FinalityCertificate, error) //perm:read
	// F3GetLatestCertificate returns the latest finality certificate
	F3GetLatestCertificate(ctx context.Context) (*certs.FinalityCertificate, error) //perm:read
}
