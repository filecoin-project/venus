package cmd

import (
	"bufio"
	"context"
	"github.com/filecoin-project/venus/pkg/config"
	cbor "github.com/ipfs/go-ipld-cbor"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-homedir"
	xerrors "github.com/pkg/errors"
	"gopkg.in/cheggaaa/pb.v1"
)

var logImport = logging.Logger("commands/import")

// Import cache tipset cids to store.
// The value of the cached tipset CIDS is used as the check-point when running `venus daemon`
func Import(r repo.Repo, fileName string) error {
	return importChain(r, fileName)
}

func importChain(r repo.Repo, fname string) error {
	var rd io.Reader
	var l int64
	if strings.HasPrefix(fname, "http://") || strings.HasPrefix(fname, "https://") {
		resp, err := http.Get(fname) //nolint:gosec
		if err != nil {
			return err
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusOK {
			return xerrors.Errorf("non-200 response: %d", resp.StatusCode)
		}

		rd = resp.Body
		l = resp.ContentLength
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
	ipldCborStore := cbor.NewCborStore(bs)
	chainStore := chain.NewStore(r.ChainDatastore(), ipldCborStore, bs, config.DefaultForkUpgradeParam, cid.Undef)

	bufr := bufio.NewReaderSize(rd, 1<<20)

	bar := pb.New64(l)
	br := bar.NewProxyReader(bufr)
	bar.ShowTimeLeft = true
	bar.ShowPercent = true
	bar.ShowSpeed = true
	bar.Units = pb.U_BYTES

	bar.Start()
	tip, err := chainStore.Import(br)
	if err != nil {
		return xerrors.Errorf("importing chain failed: %s", err)
	}
	bar.Finish()

	err = chainStore.SetHead(context.TODO(), tip)
	if err != nil {
		return xerrors.Errorf("importing chain failed: %s", err)
	}
	logImport.Infof("accepting %s as new head", tip.Key().String())

	err = chainStore.WriteCheckPoint(context.TODO(), tip.Key())
	if err != nil {
		logImport.Errorf("set check point error: %s", err.Error())
	}

	return err
}
