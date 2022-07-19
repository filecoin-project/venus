package cmd

import (
	"bytes"
	"fmt"
	"os"

	"github.com/filecoin-project/venus/cmd/tablewriter"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-car"
)

var cidCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Cid command",
	},
	Subcommands: map[string]*cmds.Command{
		"inspect-bundle": inspectBundleCmd,
	},
}

var inspectBundleCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get the manifest CID from a car file, as well as the actor code CIDs",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("path", true, false, ""),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context

		f, err := os.OpenFile(req.Arguments[0], os.O_RDONLY, 0664)
		if err != nil {
			return fmt.Errorf("opening the car file: %w", err)
		}

		bs := blockstoreutil.NewMemory()
		wrapBs := adt.WrapStore(ctx, cbor.NewCborStore(bs))

		hdr, err := car.LoadCar(ctx, bs, f)
		if err != nil {
			return fmt.Errorf("error loading car file: %w", err)
		}

		manifestCid := hdr.Roots[0]

		if err := re.Emit("Manifest CID: " + manifestCid.String()); err != nil {
			return err
		}

		entries, err := actors.ReadManifest(ctx, wrapBs, manifestCid)
		if err != nil {
			return fmt.Errorf("error loading manifest: %w", err)
		}

		buf := &bytes.Buffer{}
		tw := tablewriter.New(tablewriter.Col("Actor"), tablewriter.Col("CID"))
		for name, cid := range entries {
			tw.Write(map[string]interface{}{
				"Actor": name,
				"CID":   cid.String(),
			})
		}
		if err := tw.Flush(buf); err != nil {
			return err
		}

		return re.Emit(buf)
	},
}
