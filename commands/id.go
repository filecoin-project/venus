package commands

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// IDDetails is a collection of information about a node.
type IDDetails struct {
	Addresses       []ma.Multiaddr
	ID              peer.ID
	AgentVersion    string
	ProtocolVersion string
	PublicKey       []byte // raw bytes
}

var idCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show info about the network peers",
	},
	Options: []cmdkit.Option{
		// TODO: ideally copy this from the `ipfs id` command
		cmdkit.StringOption("format", "f", "Specify an output format"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addrs := GetPorcelainAPI(env).NetworkGetPeerAddresses()
		hostID := GetPorcelainAPI(env).NetworkGetPeerID()

		details := IDDetails{
			Addresses: make([]ma.Multiaddr, len(addrs)),
			ID:        hostID,
		}

		for i, addr := range addrs {
			subaddr, err := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", hostID.Pretty()))
			if err != nil {
				return err
			}
			details.Addresses[i] = addr.Encapsulate(subaddr)
		}

		return re.Emit(&details)
	},
	Type: IDDetails{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, val *IDDetails) error {
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

func idFormatSubstitute(format string, val *IDDetails) string {
	addrStrings := make([]string, len(val.Addresses))
	for i, addr := range val.Addresses {
		addrStrings[i] = addr.String()
	}
	output := format
	output = strings.Replace(output, "<id>", val.ID.Pretty(), -1)
	output = strings.Replace(output, "<aver>", val.AgentVersion, -1)
	output = strings.Replace(output, "<pver>", val.ProtocolVersion, -1)
	output = strings.Replace(output, "<pubkey>", base64.StdEncoding.EncodeToString(val.PublicKey), -1)
	output = strings.Replace(output, "<addrs>", strings.Join(addrStrings, "\n"), -1)
	output = strings.Replace(output, "\\n", "\n", -1)
	output = strings.Replace(output, "\\t", "\t", -1)
	return output
}

// MarshalJSON implements json.Marshaler
func (idd IDDetails) MarshalJSON() ([]byte, error) {
	addressStrings := make([]string, len(idd.Addresses))
	for i, addr := range idd.Addresses {
		addressStrings[i] = addr.String()
	}

	v := map[string]interface{}{
		"Addresses": addressStrings,
	}

	if idd.ID != "" {
		v["ID"] = idd.ID.Pretty()
	}
	if idd.AgentVersion != "" {
		v["AgentVersion"] = idd.AgentVersion
	}
	if idd.ProtocolVersion != "" {
		v["ProtocolVersion"] = idd.ProtocolVersion
	}
	if idd.PublicKey != nil {
		// Base64-encode the public key explicitly.
		// This is what the built-in JSON encoder does to []byte too.
		v["PublicKey"] = base64.StdEncoding.EncodeToString(idd.PublicKey)
	}
	return json.Marshal(v)
}

// UnmarshalJSON implements Unmarshaler
func (idd *IDDetails) UnmarshalJSON(data []byte) error {
	var v map[string]*json.RawMessage
	var err error
	if err = json.Unmarshal(data, &v); err != nil {
		return err
	}

	var addresses []string
	if err := decode(v, "Addresses", &addresses); err != nil {
		return err
	}
	idd.Addresses = make([]ma.Multiaddr, len(addresses))
	for i, addr := range addresses {
		a, err := ma.NewMultiaddr(addr)
		if err != nil {
			return err
		}
		idd.Addresses[i] = a
	}

	var id string
	if err := decode(v, "ID", &id); err != nil {
		return err
	}
	if idd.ID, err = peer.IDB58Decode(id); err != nil {
		return err
	}

	if err := decode(v, "AgentVersion", &idd.AgentVersion); err != nil {
		return err
	}
	if err := decode(v, "ProtocolVersion", &idd.ProtocolVersion); err != nil {
		return err
	}
	if err := decode(v, "PublicKey", &idd.PublicKey); err != nil {
		return err
	}
	return nil
}

func decode(idd map[string]*json.RawMessage, key string, dest interface{}) error {
	if raw := idd[key]; raw != nil {
		if err := json.Unmarshal(*raw, &dest); err != nil {
			return err
		}
	}
	return nil
}
