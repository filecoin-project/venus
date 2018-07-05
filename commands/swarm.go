package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"

	swarm "gx/ipfs/QmSwZMWwFZSUpe5muU2xgTUwppH24KfMwdPXiwbEp2c6G5/go-libp2p-swarm"
	cmds "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/filnet"
)

// COPIED FROM go-ipfs core/commands/swarm.go
// TODO a lot of this functionality should migrate to the filnet package.

// swarmCmd contains swarm commands.
var swarmCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with the swarm.",
		ShortDescription: `
'go-filecoin swarm' is a tool to manipulate the libp2p swarm. The swarm is the
component that opens, listens for, and maintains connections to other
libp2p peers on the internet.
`,
	},
	Subcommands: map[string]*cmds.Command{
		//"addrs":      swarmAddrsCmd,
		"connect": swarmConnectCmd,
		//"disconnect": swarmDisconnectCmd,
		//"filters":    swarmFiltersCmd,
		"peers": swarmPeersCmd,
	},
}

type streamInfo struct {
	Protocol string
}

type connInfo struct {
	Addr    string
	Peer    string
	Latency string
	Muxer   string
	Streams []streamInfo
}

func (ci *connInfo) Less(i, j int) bool {
	return ci.Streams[i].Protocol < ci.Streams[j].Protocol
}

func (ci *connInfo) Len() int {
	return len(ci.Streams)
}

func (ci *connInfo) Swap(i, j int) {
	ci.Streams[i], ci.Streams[j] = ci.Streams[j], ci.Streams[i]
}

type connInfos struct {
	Peers []connInfo
}

func (ci connInfos) Less(i, j int) bool {
	return ci.Peers[i].Addr < ci.Peers[j].Addr
}

func (ci connInfos) Len() int {
	return len(ci.Peers)
}

func (ci connInfos) Swap(i, j int) {
	ci.Peers[i], ci.Peers[j] = ci.Peers[j], ci.Peers[i]
}

var swarmPeersCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List peers with open connections.",
		ShortDescription: `
'ipfs swarm peers' lists the set of peers this node is connected to.
`,
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", "v", "display all extra information"),
		cmdkit.BoolOption("streams", "Also list information about open streams for each peer"),
		cmdkit.BoolOption("latency", "Also list information about latency to each peer"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {

		n := GetNode(env)

		if n.Host == nil {
			return ErrNodeOffline
		}

		verbose, _ := req.Options["verbose"].(bool)
		latency, _ := req.Options["latency"].(bool)
		streams, _ := req.Options["streams"].(bool)

		conns := n.Host.Network().Conns()

		var out connInfos
		for _, c := range conns {
			pid := c.RemotePeer()
			addr := c.RemoteMultiaddr()

			ci := connInfo{
				Addr: addr.String(),
				Peer: pid.Pretty(),
			}

			swcon, ok := c.(*swarm.Conn)
			if ok {
				ci.Muxer = fmt.Sprintf("%T", swcon.StreamConn().Conn())
			}

			if verbose || latency {
				lat := n.Host.Peerstore().LatencyEWMA(pid)
				if lat == 0 {
					ci.Latency = "n/a"
				} else {
					ci.Latency = lat.String()
				}
			}
			if verbose || streams {
				strs, err := c.GetStreams()
				if err != nil {
					return err
				}

				for _, s := range strs {
					ci.Streams = append(ci.Streams, streamInfo{Protocol: string(s.Protocol())})
				}
			}
			sort.Sort(&ci)
			out.Peers = append(out.Peers, ci)
		}

		sort.Sort(&out)
		re.Emit(&out) // nolint: errcheck

		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ci *connInfos) error {
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
	Type: connInfos{},
}

type connectResult struct {
	Peer    string
	Success bool
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
		ctx := req.Context

		n := GetNode(env)

		addrs := req.Arguments

		snet, ok := n.Host.Network().(*swarm.Network)
		if !ok {
			return fmt.Errorf("peerhost network was not swarm")
		}

		swrm := snet.Swarm()

		pis, err := filnet.PeerAddrsToPeerInfos(addrs)
		if err != nil {
			return err
		}

		output := make([]connectResult, len(pis))
		for i, pi := range pis {
			swrm.Backoff().Clear(pi.ID)

			output[i].Peer = pi.ID.Pretty()

			err := n.Host.Connect(ctx, pi)
			if err != nil {
				return fmt.Errorf("%s failure: %s", output[i].Peer, err)
			}
		}

		re.Emit(output) // nolint: errcheck

		return nil
	},
	Type: []connectResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *[]connectResult) error {
			for _, a := range *res {
				fmt.Fprintf(w, "connect %s success\n", a.Peer) // nolint: errcheck
			}
			return nil
		}),
	},
}
