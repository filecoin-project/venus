package commands

import (
	"fmt"
	"io"

	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/proofs/libsectorbuilder"
	"github.com/filecoin-project/go-filecoin/types"
)

var protocolCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show protocol parameter details",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		params, err := GetPorcelainAPI(env).ProtocolParameters(env.Context())
		if err != nil {
			return err
		}
		return re.Emit(params)
	},
	Type: porcelain.ProtocolParams{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, pp *porcelain.ProtocolParams) error {
			_, err := fmt.Fprintf(w, "Auto-Seal Interval: %d seconds\nSector Sizes:\n", pp.AutoSealInterval)
			if err != nil {
				return err
			}

			sectorSize := types.OneKiBSectorSize
			if pp.ProofsMode == types.LiveProofsMode {
				sectorSize = types.TwoHundredFiftySixMiBSectorSize
			}

			maxUserBytes := types.NewBytesAmount(libsectorbuilder.GetMaxUserBytesPerStagedSector(sectorSize.Uint64()))

			_, err = fmt.Fprintf(w, "\t%s (%s writeable)\n", readableBytesAmount(float64(sectorSize.Uint64())), readableBytesAmount(float64(maxUserBytes.Uint64())))
			if err != nil {
				return err
			}

			return nil
		}),
	},
}

func readableBytesAmount(amt float64) string {
	unit := 0
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"}

	for amt >= 1024 && unit < len(units)-1 {
		amt /= 1024
		unit++
	}

	return fmt.Sprintf("%.2f %s", amt, units[unit])
}
