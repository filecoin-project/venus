package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipld/go-car"
	"golang.org/x/xerrors"
)

var cidCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Cid command",
	},
	Subcommands: map[string]*cmds.Command{
		"manifest-cid-from-car": cidFromCarCmd,
	},
}

var cidFromCarCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get the manifest CID from a car file",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, ""),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context

		f, err := os.OpenFile(req.Arguments[0], os.O_RDONLY, 0664)
		if err != nil {
			return xerrors.Errorf("opening the car file: %w", err)
		}

		bs := blockstoreutil.NewMemory()
		if err != nil {
			return err
		}

		data, err := io.ReadAll(f)
		if err != nil {
			return xerrors.Errorf("error reading car file: %w", err)
		}

		blobr := bytes.NewReader(data)

		hdr, err := car.LoadCar(ctx, bs, blobr)
		if err != nil {
			return xerrors.Errorf("error loading car file: %w", err)
		}

		manifestCid := hdr.Roots[0]

		fmt.Printf("Manifest CID: %s\n", manifestCid.String())

		return nil
	},
	Type: "",
}
