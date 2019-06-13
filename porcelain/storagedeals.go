package porcelain

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore/query"
	cbor "github.com/ipfs/go-ipld-cbor"
	errors "github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/types"
)

var (
	// ErrDealNotFound means DealGet failed to find a matching deal
	ErrDealNotFound = errors.New("deal not found")
)

// StorageDealLsResult represents a result from the storage deal Ls method. This
// can either be an error or a storage deal.
type StorageDealLsResult struct {
	Deal storagedeal.Deal
	Err  error
}

type dealGetPlumbing interface {
	DealsLs(context.Context) (<-chan *StorageDealLsResult, error)
}

// DealGet returns a single deal matching a given cid or an error
func DealGet(ctx context.Context, plumbing dealGetPlumbing, dealCid cid.Cid) (*storagedeal.Deal, error) {
	dealCh, err := plumbing.DealsLs(ctx)
	if err != nil {
		return nil, err
	}
	for deal := range dealCh {
		if deal.Err != nil {
			return nil, deal.Err
		}
		if deal.Deal.Response.ProposalCid == dealCid {
			return &deal.Deal, nil
		}
	}
	return nil, ErrDealNotFound
}

type dealLsPlumbing interface {
	ConfigGet(string) (interface{}, error)
	DealsIterator() (*query.Results, error)
}

// DealsLs returns an channel with all deals or a possible error
func DealsLs(ctx context.Context, plumbing dealLsPlumbing) (<-chan *StorageDealLsResult, error) {
	out := make(chan *StorageDealLsResult)
	results, err := plumbing.DealsIterator()
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(out)
		for entry := range (*results).Next() {
			select {
			case <-ctx.Done():
				out <- &StorageDealLsResult{
					Err: ctx.Err(),
				}
				return
			default:
				var storageDeal storagedeal.Deal
				if err := cbor.DecodeInto(entry.Value, &storageDeal); err != nil {
					out <- &StorageDealLsResult{
						Err: errors.Wrap(err, "failed to unmarshal deals from datastore"),
					}
					return
				}
				out <- &StorageDealLsResult{
					Deal: storageDeal,
				}
			}
		}
	}()

	return out, nil
}

type dealRedeemPlumbing interface {
	ChainBlockHeight() (*types.BlockHeight, error)
	DealGet(context.Context, cid.Cid) (*storagedeal.Deal, error)
	MessagePreview(context.Context, address.Address, address.Address, string, ...interface{}) (types.GasUnits, error)
	MessageSend(context.Context, address.Address, address.Address, types.AttoFIL, types.AttoFIL, types.GasUnits, string, ...interface{}) (cid.Cid, error)
}

// DealRedeem redeems a voucher for the deal with the given cid and returns
// either the cid of the created redeem message or an error
func DealRedeem(ctx context.Context, plumbing dealRedeemPlumbing, fromAddr address.Address, dealCid cid.Cid, gasPrice types.AttoFIL, gasLimit types.GasUnits) (cid.Cid, error) {
	params, err := buildDealRedeemParams(ctx, plumbing, dealCid)
	if err != nil {
		return cid.Undef, err
	}

	return plumbing.MessageSend(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		types.NewAttoFILFromFIL(0),
		gasPrice,
		gasLimit,
		"redeem",
		params...,
	)
}

// DealRedeemPreview previews the redeem method for a deal and returns the
// expected gas used
func DealRedeemPreview(ctx context.Context, plumbing dealRedeemPlumbing, fromAddr address.Address, dealCid cid.Cid) (types.GasUnits, error) {
	params, err := buildDealRedeemParams(ctx, plumbing, dealCid)
	if err != nil {
		return types.NewGasUnits(0), err
	}

	return plumbing.MessagePreview(
		ctx,
		fromAddr,
		address.PaymentBrokerAddress,
		"redeem",
		params...,
	)
}

func buildDealRedeemParams(ctx context.Context, plumbing dealRedeemPlumbing, dealCid cid.Cid) ([]interface{}, error) {
	deal, err := plumbing.DealGet(ctx, dealCid)
	if err != nil {
		return []interface{}{}, err
	}

	currentBlockHeight, err := plumbing.ChainBlockHeight()
	if err != nil {
		return []interface{}{}, err
	}

	var voucher *types.PaymentVoucher
	for _, v := range deal.Proposal.Payment.Vouchers {
		if currentBlockHeight.LessThan(&v.ValidAt) {
			continue
		}
		if voucher != nil && v.Amount.LessThan(voucher.Amount) {
			continue
		}
		voucher = v
	}

	if voucher == nil {
		return []interface{}{}, errors.New("no remaining redeemable vouchers found")
	}

	return []interface{}{
		voucher.Payer,
		&voucher.Channel,
		voucher.Amount,
		&voucher.ValidAt,
		voucher.Condition,
		[]byte(voucher.Signature),
		[]interface{}{},
	}, nil
}
