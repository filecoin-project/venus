package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/filecoin-project/venus/pkg/consensus/chainselector"
	"github.com/filecoin-project/venus/pkg/httpreader"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	logging "github.com/ipfs/go-log/v2"
	"github.com/klauspost/compress/zstd"
	"github.com/mitchellh/go-homedir"
	"gopkg.in/cheggaaa/pb.v1"
)

var logImport = logging.Logger("commands/import")

// Import cache tipset cids to store.
// The value of the cached tipset CIDS is used as the check-point when running `venus daemon`
func Import(ctx context.Context, r repo.Repo, network string, fileName string) error {
	return importChain(ctx, r, network, fileName)
}

func importChain(ctx context.Context, r repo.Repo, network string, fname string) error {
	var rd io.Reader
	var l int64
	if strings.HasPrefix(fname, "http://") || strings.HasPrefix(fname, "https://") {
		rrd, err := httpreader.NewResumableReader(ctx, fname)
		if err != nil {
			return fmt.Errorf("fetching chain CAR failed: setting up resumable reader: %w", err)
		}

		rd = rrd
		l = rrd.ContentLength()
	} else {
		fname, err := homedir.Expand(fname)
		if err != nil {
			return err
		}

		fi, err := os.Open(fname)
		if err != nil {
			return err
		}
		defer fi.Close() //nolint:errcheck

		st, err := os.Stat(fname)
		if err != nil {
			return err
		}

		rd = fi
		l = st.Size()
	}

	bs := r.Datastore()
	// setup a ipldCbor on top of the local store
	chainStore := chain.NewStore(r.ChainDatastore(), bs, cid.Undef, chainselector.Weight)

	bufr := bufio.NewReaderSize(rd, 1<<20)

	header, err := bufr.Peek(4)
	if err != nil {
		return fmt.Errorf("peek header: %w", err)
	}

	bar := pb.New64(l)
	br := bar.NewProxyReader(bufr)
	bar.ShowTimeLeft = true
	bar.ShowPercent = true
	bar.ShowSpeed = true
	bar.Units = pb.U_BYTES

	var ir io.Reader = br
	if string(header[1:]) == "\xB5\x2F\xFD" { // zstd
		zr, err := zstd.NewReader(br)
		if err != nil {
			return err
		}
		defer zr.Close()
		ir = zr
	}

	f3Ds := namespace.Wrap(r.MetaDatastore(), datastore.NewKey("/f3"))
	if err != nil {
		return fmt.Errorf("failed to open f3 datastore: %w", err)
	}

	bar.Start()
	tip, genesisBlk, err := chainStore.Import(ctx, network, f3Ds, ir)
	if err != nil {
		return fmt.Errorf("importing chain failed: %s", err)
	}
	bar.Finish()

	err = chainStore.SetHead(context.TODO(), tip)
	if err != nil {
		return fmt.Errorf("importing chain failed: %s", err)
	}
	logImport.Infof("accepting %s as new head", tip.Key().String())

	if err := chainStore.PersistGenesisCID(ctx, genesisBlk); err != nil {
		return fmt.Errorf("persist genesis failed: %v", err)
	}

	return err
}
