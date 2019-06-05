package fast

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-files"
)

// ClientCat runs the client cat command against the filecoin process.
// A ReadCloser is returned representing the data.
// TODO(frrist): address buffering in filecoin plugins to exert appropriate backpressure on the
// reader IPTB returns.
func (f *Filecoin) ClientCat(ctx context.Context, cid cid.Cid) (io.ReadCloser, error) {
	out, err := f.RunCmdWithStdin(ctx, nil, "go-filecoin", "client", "cat", cid.String())
	if err != nil {
		return nil, err
	}
	return out.Stdout(), err
}

// ClientImport runs the client import data command against the filecoin process.
func (f *Filecoin) ClientImport(ctx context.Context, data files.File) (cid.Cid, error) {
	var out cid.Cid
	if err := f.RunCmdJSONWithStdin(ctx, data, &out, "go-filecoin", "client", "import"); err != nil {
		return cid.Undef, err
	}
	return out, nil
}

// ClientProposeStorageDeal runs the client propose-storage-deal command against the filecoin process.
func (f *Filecoin) ClientProposeStorageDeal(ctx context.Context, data cid.Cid,
	miner address.Address, ask uint64, duration uint64, allowDuplicates bool) (*storagedeal.Response, error) {

	var out storagedeal.Response
	sData := data.String()
	sMiner := miner.String()
	sAsk := fmt.Sprintf("%d", ask)
	sDuration := fmt.Sprintf("%d", duration)

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "client", "propose-storage-deal", sMiner, sData, sAsk, sDuration); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClientQueryStorageDeal runs the client query-storage-deal command against the filecoin process.
func (f *Filecoin) ClientQueryStorageDeal(ctx context.Context, prop cid.Cid) (*storagedeal.Response, error) {
	var out storagedeal.Response

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "client", "query-storage-deal", prop.String()); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClientListAsks runs the client list-asks command against the filecoin process.
// A json decoer is returned that asks may be decoded from.
func (f *Filecoin) ClientListAsks(ctx context.Context) (*json.Decoder, error) {
	return f.RunCmdLDJSONWithStdin(ctx, nil, "go-filecoin", "client", "list-asks")
}
