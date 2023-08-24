package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/filecoin-project/venus/pkg/httpreader"

	"github.com/DataDog/zstd"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-homedir"
	"gopkg.in/cheggaaa/pb.v1"
)

var logImport = logging.Logger("commands/import")

// Import cache tipset cids to store.
// The value of the cached tipset CIDS is used as the check-point when running `venus daemon`
func Import(ctx context.Context, r repo.Repo, fileName string) error {
	return importChain(ctx, r, fileName)
}

func importChain(ctx context.Context, r repo.Repo, fname string) error {
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
	chainStore := chain.NewStore(r.ChainDatastore(), bs, cid.Undef, chain.NewMockCirculatingSupplyCalculator())

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
		zr := zstd.NewReader(br)
		defer func() {
			if err := zr.Close(); err != nil {
				log.Errorw("closing zstd reader", "error", err)
			}
		}()
		ir = zr
	}

	bar.Start()
	tip, err := chainStore.Import(ctx, ir)
	if err != nil {
		return fmt.Errorf("importing chain failed: %s", err)
	}
	bar.Finish()

	err = chainStore.SetHead(context.TODO(), tip)
	if err != nil {
		return fmt.Errorf("importing chain failed: %s", err)
	}
	logImport.Infof("accepting %s as new head", tip.Key().String())

	genesis, err := chainStore.GetTipSetByHeight(ctx, tip, 0, false)
	if err != nil {
		return fmt.Errorf("got genesis failed: %v", err)
	}
	if err := chainStore.PersistGenesisCID(ctx, genesis.Blocks()[0]); err != nil {
		return fmt.Errorf("persist genesis failed: %v", err)
	}

	err = chainStore.WriteCheckPoint(context.TODO(), tip.Key())
	if err != nil {
		logImport.Errorf("set check point error: %s", err.Error())
	}

	return err
}
