package commands

import (
	"fmt"
	"io"
	"strings"

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	peer "gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	cmds "gx/ipfs/Qmf46mr235gtyxizkKUkTH5fo62Thza2zwXR4DWC7rkoqF/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/api"
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
		"connect":  swarmConnectCmd,
		"peers":    swarmPeersCmd,
		"findpeer": findPeerDhtCmd,
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

		out, err := GetAPI(env).Swarm().Peers(req.Context, verbose, latency, streams)
		if err != nil {
			return err
		}

		return re.Emit(&out)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ci *api.SwarmConnInfos) error {
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
	Type: api.SwarmConnInfos{},
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
		out, err := GetAPI(env).Swarm().Connect(req.Context, req.Arguments)
		if err != nil {
			return err
		}

		return re.Emit(out)
	},
	Type: []api.SwarmConnectResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *[]api.SwarmConnectResult) error {
			for _, a := range *res {
				fmt.Fprintf(w, "connect %s success\n", a.Peer) // nolint: errcheck
			}
			return nil
		}),
	},
}

var findPeerDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Find the multiaddresses associated with a Peer ID.",
		ShortDescription: "Outputs a list of newline-delimited multiaddresses.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peerID", true, false, "The ID of the peer to search for."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		peerID, err := peer.IDB58Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		out, err := GetAPI(env).Swarm().FindPeer(req.Context, peerID)
		if err != nil {
			return err
		}

		for _, addr := range out.Addrs {
			if err := res.Emit(addr.String()); err != nil {
				return err
			}
		}
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, addr string) error {
			_, err := fmt.Fprintln(w, addr)
			return err
		}),
	},
}
