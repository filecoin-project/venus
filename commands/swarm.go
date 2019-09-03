package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/go-filecoin/net"
)

// swarmCmd contains swarm commands.
var swarmCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with the swarm",
		ShortDescription: `
'go-filecoin swarm' is a tool to manipulate the libp2p swarm. The swarm is the
component that opens, listens for, and maintains connections to other
libp2p peers on the internet.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"connect": swarmConnectCmd,
		"peers":   swarmPeersCmd,
	},
}

var swarmPeersCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List peers with open connections.",
		ShortDescription: `
'go-filecoin swarm peers' lists the set of peers this node is connected to.
`,
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", "v", "Display all extra information"),
		cmdkit.BoolOption("streams", "Also list information about open streams for each peer"),
		cmdkit.BoolOption("latency", "Also list information about latency to each peer"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		verbose, _ := req.Options["verbose"].(bool)
		latency, _ := req.Options["latency"].(bool)
		streams, _ := req.Options["streams"].(bool)

		out, err := GetPorcelainAPI(env).NetworkPeers(req.Context, verbose, latency, streams)
		if err != nil {
			return err
		}

		return re.Emit(&out)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ci *net.SwarmConnInfos) error {
			pipfs := ma.ProtocolWithCode(ma.P_IPFS).Name
			for _, info := range ci.Peers {
				ids := fmt.Sprintf("/%s/%s", pipfs, info.Peer)
				if strings.HasSuffix(info.Addr, ids) {
					fmt.Fprintf(w, "%s", info.Addr) // nolint: errcheck
				} else {
					fmt.Fprintf(w, "%s%s", info.Addr, ids) // nolint: errcheck
				}
				if info.Latency != "" {
					fmt.Fprintf(w, " %s", info.Latency) // nolint: errcheck
				}
				fmt.Fprintln(w) // nolint: errcheck

				for _, s := range info.Streams {
					if s.Protocol == "" {
						s.Protocol = "<no protocol name>"
					}

					fmt.Fprintf(w, "  %s\n", s.Protocol) // nolint: errcheck
				}
			}

			return nil
		}),
	},
	Type: net.SwarmConnInfos{},
}

var swarmConnectCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Open connection to a given address.",
		ShortDescription: `
'go-filecoin swarm connect' opens a new direct connection to a peer address.

The address format is a multiaddr:

go-filecoin swarm connect /ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", true, true, "Address of peer to connect to.").EnableStdin(),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		results, err := GetPorcelainAPI(env).NetworkConnect(req.Context, req.Arguments)
		if err != nil {
			return err
		}

		for result := range results {
			if result.Err != nil {
				return result.Err
			}
			if err := re.Emit(result.PeerID); err != nil {
				return err
			}
		}

		return nil
	},
	Type: peer.ID(""),
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, result peer.ID) error {
			fmt.Fprintf(w, "connect %s success\n", result.Pretty()) // nolint: errcheck
			return nil
		}),
	},
}
