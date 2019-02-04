package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	cmds "gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/api"
)

var idCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show info about the network peers",
	},
	Options: []cmdkit.Option{
		// TODO: ideally copy this from the `ipfs id` command
		cmdkit.StringOption("format", "f", "Specify an output format"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		details, err := GetAPI(env).ID().Details()
		if err != nil {
			return err
		}
		return re.Emit(details)
	},
	Type: api.IDDetails{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, val *api.IDDetails) error {
			format, found := req.Options["format"].(string)
			if found {
				output := idFormatSubstitute(format, val)
				_, err := fmt.Fprint(w, output)
				return err
			}

			marshaled, err := json.MarshalIndent(val, "", "\t")
			if err != nil {
				return err
			}
			marshaled = append(marshaled, byte('\n'))

			_, err = w.Write(marshaled)
			return err
		}),
	},
}

func idFormatSubstitute(format string, val *api.IDDetails) string {
	output := format
	output = strings.Replace(output, "<id>", val.ID.Pretty(), -1)
	output = strings.Replace(output, "<aver>", val.AgentVersion, -1)
	output = strings.Replace(output, "<pver>", val.ProtocolVersion, -1)
	output = strings.Replace(output, "<pubkey>", val.PublicKey, -1)
	output = strings.Replace(output, "<addrs>", strings.Join(val.Addresses, "\n"), -1)
	output = strings.Replace(output, "\\n", "\n", -1)
	output = strings.Replace(output, "\\t", "\t", -1)
	return output
}
