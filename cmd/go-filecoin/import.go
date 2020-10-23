package commands

import (
	"bufio"
	"context"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-homedir"
	xerrors "github.com/pkg/errors"
	"gopkg.in/cheggaaa/pb.v1"
	"io"
	"net/http"
	"os"
	"strings"
)

var logImport = logging.Logger("commands/import")

var importCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Initialize a filecoin repo",
	},
	Options: []cmds.Option{
		cmds.StringOption("path", "path of file or HTTP(S) URL containing archive of genesis block DAG data"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		repoDir, _ := req.Options[OptionRepoDir].(string)
		repoDir, err := paths.GetRepoPath(repoDir)
		if err != nil {
			return err
		}
		rep, err := repo.OpenFSRepo(repoDir, repo.Version)
		if err != nil {
			return err
		}
		importPath, _ := req.Options["path"].(string)

		return ImportChain(rep, importPath)
	},
}

func ImportChain(r repo.Repo, fname string) error {
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

	chainStatusReporter := chain.NewStatusReporter()
	bs := blockstore.NewBlockstore(r.Datastore())
	// setup a ipldCbor on top of the local store
	ipldCborStore := cborutil.NewIpldStore(bs)
	chainStore := chain.NewStore(r.ChainDatastore(), ipldCborStore, bs, chainStatusReporter, block.UndefTipSet.Key(), cid.Undef)

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
		return xerrors.Errorf("importing chain failed: %w", err)
	}
	bar.Finish()

	chainStore.SetHead(context.TODO(), tip)
	logImport.Infof("accepting %s as new head", tip.Key().String())
	return chainStore.SetHead(context.Background(), tip)
}
