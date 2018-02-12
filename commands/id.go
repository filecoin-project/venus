package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/node"
)

// idOutput is the output of the /id api endpoint
type idOutput struct {
	Addresses       []string
	ID              string
	AgentVersion    string
	ProtocolVersion string
	PublicKey       string
}

func idOutputFromNode(fcn *node.Node) *idOutput {
	var out idOutput
	for _, a := range fcn.Host.Addrs() {
		out.Addresses = append(out.Addresses, fmt.Sprintf("%s/ipfs/%s", a, fcn.Host.ID().Pretty()))
	}
	out.ID = fcn.Host.ID().Pretty()
	return &out
}

var idCmd = &cmds.Command{
	Options: []cmdkit.Option{
		// TODO: ideally copy this from the `ipfs id` command
		cmdkit.StringOption("format", "f", "specify an output format"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)

		out := idOutputFromNode(fcn)

		re.Emit(out) // nolint: errcheck
	},
	Type: idOutput{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, val *idOutput) error {
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

func idFormatSubstitute(format string, val *idOutput) string {
	output := format
	output = strings.Replace(output, "<id>", val.ID, -1)
	output = strings.Replace(output, "<aver>", val.AgentVersion, -1)
	output = strings.Replace(output, "<pver>", val.ProtocolVersion, -1)
	output = strings.Replace(output, "<pubkey>", val.PublicKey, -1)
	output = strings.Replace(output, "<addrs>", strings.Join(val.Addresses, "\n"), -1)
	output = strings.Replace(output, "\\n", "\n", -1)
	output = strings.Replace(output, "\\t", "\t", -1)
	return output
}
