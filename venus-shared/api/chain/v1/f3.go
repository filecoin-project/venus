package v1

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-f3/certs"
	"github.com/filecoin-project/go-f3/gpbft"
	"github.com/filecoin-project/go-f3/manifest"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type IF3 interface {
	//*********************************** ALL F3 APIs below are not stable & subject to change ***********************************

	// F3GetOrRenewParticipationTicket retrieves or renews a participation ticket
	// necessary for a miner to engage in the F3 consensus process for the given
	// number of instances.
	//
	// This function accepts an optional previous ticket. If provided, a new ticket
	// will be issued only under one the following conditions:
	//   1. The previous ticket has expired.
	//   2. The issuer of the previous ticket matches the node processing this
	//      request.
	//
	// If there is an issuer mismatch (ErrF3ParticipationIssuerMismatch), the miner
	// must retry obtaining a new ticket to ensure it is only participating in one F3
	// instance at any time. If the number of instances is beyond the maximum leasable
	// participation instances accepted by the node ErrF3ParticipationTooManyInstances
	// is returned.
	//
	// Note: Successfully acquiring a ticket alone does not constitute participation.
	// The retrieved ticket must be used to invoke F3Participate to actively engage
	// in the F3 consensus process.
	F3GetOrRenewParticipationTicket(ctx context.Context, minerID address.Address, previous types.F3ParticipationTicket, instances uint64) (types.F3ParticipationTicket, error) //perm:sign
	// F3Participate enrolls a storage provider in the F3 consensus process using a
	// provided participation ticket. This ticket grants a temporary lease that enables
	// the provider to sign transactions as part of the F3 consensus.
	//
	// The function verifies the ticket's validity and checks if the ticket's issuer
	// aligns with the current node. If there is an issuer mismatch
	// (ErrF3ParticipationIssuerMismatch), the provider should retry with the same
	// ticket, assuming the issue is due to transient network problems or operational
	// deployment conditions. If the ticket is invalid
	// (ErrF3ParticipationTicketInvalid) or has expired
	// (ErrF3ParticipationTicketExpired), the provider must obtain a new ticket by
	// calling F3GetOrRenewParticipationTicket.
	//
	// The start instance associated to the given ticket cannot be less than the
	// start instance of any existing lease held by the miner. Otherwise,
	// ErrF3ParticipationTicketStartBeforeExisting is returned. In this case, the
	// miner should acquire a new ticket before attempting to participate again.
	//
	// For details on obtaining or renewing a ticket, see F3GetOrRenewParticipationTicket.
	F3Participate(ctx context.Context, ticket types.F3ParticipationTicket) (types.F3ParticipationLease, error) //perm:sign
	// F3GetCertificate returns a finality certificate at given instance number
	F3GetCertificate(ctx context.Context, instance uint64) (*certs.FinalityCertificate, error) //perm:read
	// F3GetLatestCertificate returns the latest finality certificate
	F3GetLatestCertificate(ctx context.Context) (*certs.FinalityCertificate, error) //perm:read
	// F3GetECPowerTable returns a F3 specific power table for use in standalone F3 nodes.
	F3GetECPowerTable(ctx context.Context, tsk types.TipSetKey) (gpbft.PowerEntries, error) //perm:read
	// F3GetF3PowerTable returns a F3 specific power table.
	F3GetF3PowerTable(ctx context.Context, tsk types.TipSetKey) (gpbft.PowerEntries, error) //perm:read
	// F3GetManifest returns the current manifest being used for F3
	F3GetManifest(ctx context.Context) (*manifest.Manifest, error) //perm:read
	// F3GetPowerTableByInstance returns the power table (committee) used to validate the specified instance.
	F3GetPowerTableByInstance(ctx context.Context, instance uint64) (gpbft.PowerEntries, error) //perm:read
	// F3IsRunning returns true if the F3 instance is running, false if it's not running but
	// it's enabled, and an error when disabled entirely.
	F3IsRunning(ctx context.Context) (bool, error) //perm:read
	// F3GetProgress returns the progress of the current F3 instance in terms of instance ID, round and phase.
	F3GetProgress(ctx context.Context) (gpbft.InstanceProgress, error) //perm:read

	// F3ListParticipants returns the list of miners that are currently participating in F3 via this node.
	F3ListParticipants(ctx context.Context) ([]types.F3Participant, error) //perm:read
}
