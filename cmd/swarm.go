package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/net"
)

const (
	dhtVerboseOptionName   = "verbose"
	numProvidersOptionName = "num-providers"
)

var netCmdLog = logging.Logger("net-cmd")

// swarmCmd contains swarm commands.
var swarmCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with the swarm",
		ShortDescription: `
'venus swarm' is a tool to manipulate the libp2p swarm. The swarm is the
component that opens, listens for, and maintains connections to other
libp2p peers on the internet.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"id":             idCmd,
		"query":          queryDhtCmd,
		"peers":          swarmPeersCmd,
		"connect":        swarmConnectCmd,
		"findpeer":       findPeerDhtCmd,
		"findprovs":      findProvidersDhtCmd,
		"bandwidth":      statsBandwidthCmd,
		"ping":           swarmPingCmd,
		"disconnect":     disconnectCmd,
		"reachability":   reachabilityCmd,
		"protect":        protectAddCmd,
		"unprotect":      protectRemoveCmd,
		"list-protected": protectListCmd,
	},
}

var swarmPeersCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List peers with open connections.",
		ShortDescription: `
'venus swarm peers' lists the set of peers this node is connected to.
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption("agent", "a", "Print agent name"),
		cmds.BoolOption("extended", "x", "Print extended peer information in json"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context
		needAgent, _ := req.Options["agent"].(bool)
		extended, _ := req.Options["extended"].(bool)

		peers, err := env.(*node.Env).NetworkAPI.NetPeers(ctx)
		if err != nil {
			return err
		}

		sort.Slice(peers, func(i, j int) bool {
			return strings.Compare(string(peers[i].ID), string(peers[j].ID)) > 0
		})

		buf := &bytes.Buffer{}
		writer := NewSilentWriter(buf)

		if extended {
			// deduplicate
			seen := make(map[peer.ID]struct{})

			for _, peer := range peers {
				_, dup := seen[peer.ID]
				if dup {
					continue
				}
				seen[peer.ID] = struct{}{}

				info, err := env.(*node.Env).NetworkAPI.NetPeerInfo(ctx, peer.ID)
				if err != nil {
					netCmdLog.Warnf("error getting extended peer info: %s", err)
				} else {
					bytes, err := json.Marshal(&info)
					if err != nil {
						netCmdLog.Warnf("error marshalling extended peer info: %s", err)
					} else {
						writer.Println(string(bytes))
					}
				}
			}
		} else {
			for _, peer := range peers {
				var agent string
				if needAgent {
					agent, err = env.(*node.Env).NetworkAPI.NetAgentVersion(ctx, peer.ID)
					if err != nil {
						netCmdLog.Warnf("getting agent version: %s", err)
					} else {
						agent = ", " + agent
					}
				}
				writer.Printf("%s, %s%s\n", peer.ID, peer.Addrs, agent)
			}
		}

		return re.Emit(buf)
	},
}

var swarmPingCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Ping peers",
		ShortDescription: `
'venus swarm ping' ping peers.
`,
	},
	Options: []cmds.Option{
		cmds.IntOption("count", "c", "specify the number of times it should ping").WithDefault(10),
		cmds.IntOption("internal", "minimum time between pings").WithDefault(1),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peers", true, true, "peers id"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) < 1 {
			return re.Emit("please provide a peerID")
		}
		ctx := req.Context
		count, _ := req.Options["count"].(int)
		interval, _ := req.Options["internal"].(int)

		pis, err := net.ParseAddresses(ctx, req.Arguments)
		if err != nil {
			return err
		}

		for _, pi := range pis {
			err := env.(*node.Env).NetworkAPI.NetConnect(ctx, pi)
			if err != nil {
				return fmt.Errorf("connect: %w", err)
			}

			var avg time.Duration
			var successful int

			for i := 0; i < count && ctx.Err() == nil; i++ {
				start := time.Now()

				rtt, err := env.(*node.Env).NetworkAPI.NetPing(ctx, pi.ID)
				if err != nil {
					if ctx.Err() != nil {
						break
					}
					log.Errorf("Ping failed: error=%v", err)
					continue
				}
				if err := re.Emit(fmt.Sprintf("Pong received: time=%v", rtt)); err != nil {
					return err
				}
				avg = avg + rtt
				successful++

				wctx, cancel := context.WithTimeout(ctx, time.Until(start.Add(time.Duration(interval)*time.Second)))
				<-wctx.Done()
				cancel()
			}

			if successful > 0 {
				if err := re.Emit(fmt.Sprintf("Average latency: %v", avg/time.Duration(successful))); err != nil {
					return err
				}
			}
		}

		return nil
	},
}

var swarmConnectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Open connection to a given address.",
		ShortDescription: `
'venus swarm connect' opens a new direct connection to a peer address.

The address format is a multiaddr:

venus swarm connect /ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, true, "address of peer to connect to.").EnableStdin(),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		pis, err := net.ParseAddresses(req.Context, req.Arguments)
		if err != nil {
			return err
		}

		for _, pi := range pis {
			err := env.(*node.Env).NetworkAPI.NetConnect(req.Context, pi)
			if err != nil {
				return err
			}
			if err := re.Emit(pi.ID.Pretty()); err != nil {
				return err
			}
		}

		return nil
	},
}

var queryDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Find the closest Peer IDs to a given Peer ID by querying the DHT.",
		ShortDescription: "Outputs a list of newline-delimited Peer IDs.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("peerID", true, false, "The peerID to run the query against."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(dhtVerboseOptionName, "v", "Print extra information."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {

		id, err := peer.Decode(req.Arguments[0])
		if err != nil {
			return cmds.ClientError("invalid peer ID")
		}

		ctx, cancel := context.WithCancel(req.Context)
		ctx, events := routing.RegisterForQueryEvents(ctx)

		closestPeers, err := env.(*node.Env).NetworkAPI.NetGetClosestPeers(ctx, string(id))
		if err != nil {
			cancel()
			return err
		}

		go func() {
			defer cancel()
			for _, p := range closestPeers {
				routing.PublishQueryEvent(ctx, &routing.QueryEvent{
					ID:   p,
					Type: routing.FinalPeer,
				})
			}
		}()

		for e := range events {
			if err := res.Emit(e); err != nil {
				return err
			}
		}

		return nil
	},
	Type: routing.QueryEvent{},
}

var findProvidersDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Find peers that can provide a given key's value.",
		ShortDescription: "Outputs a list of newline-delimited provider Peer IDs for a given key.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The key whose provider Peer IDs are output.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(dhtVerboseOptionName, "v", "Print extra information."),
		cmds.IntOption(numProvidersOptionName, "n", "The max number of providers to find.").WithDefault(20),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		numProviders, _ := req.Options[numProvidersOptionName].(int)
		if numProviders < 1 {
			return fmt.Errorf("number of providers must be greater than 0")
		}

		c, err := cid.Parse(req.Arguments[0])
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(req.Context, time.Minute)
		ctx, events := routing.RegisterForQueryEvents(ctx)

		pchan := env.(*node.Env).NetworkAPI.NetFindProvidersAsync(ctx, c, numProviders)

		go func() {
			defer cancel()
			for p := range pchan {
				np := p
				// Note that the peer IDs in these Provider
				// events are the main output of this command.
				// These results are piped back into the event
				// system so that they can be read alongside
				// other routing events which are output in
				// verbose mode but otherwise filtered.
				routing.PublishQueryEvent(ctx, &routing.QueryEvent{
					Type:      routing.Provider,
					Responses: []*peer.AddrInfo{&np},
				})
			}
		}()
		for e := range events {
			if err := res.Emit(e); err != nil {
				return err
			}
		}

		return nil
	},
	Type: routing.QueryEvent{},
}

var findPeerDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Find the multiaddresses associated with a Peer ID.",
		ShortDescription: "Outputs a list of newline-delimited multiaddresses.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peerID", true, false, "The ID of the peer to search for."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		peerID, err := peer.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		out, err := env.(*node.Env).NetworkAPI.NetFindPeer(req.Context, peerID)
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
}

var statsBandwidthCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "View bandwidth usage metrics",
	},
	Options: []cmds.Option{
		cmds.BoolOption("by-peer", "list bandwidth usage by peer"),
		cmds.BoolOption("by-protocol", "list bandwidth usage by protocol"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context
		netAPI := env.(*node.Env).NetworkAPI

		bypeer, _ := req.Options["by-peer"].(bool)
		byproto, _ := req.Options["by-protocol"].(bool)

		buf := &bytes.Buffer{}

		tw := tabwriter.NewWriter(buf, 4, 4, 2, ' ', 0)

		fmt.Fprintf(tw, "Segment\tTotalIn\tTotalOut\tRateIn\tRateOut\n")

		if bypeer {
			bw, err := netAPI.NetBandwidthStatsByPeer(ctx)
			if err != nil {
				return err
			}

			var peers []string
			for p := range bw {
				peers = append(peers, p)
			}

			sort.Slice(peers, func(i, j int) bool {
				return peers[i] < peers[j]
			})

			for _, p := range peers {
				s := bw[p]
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s/s\t%s/s\n", p, humanize.Bytes(uint64(s.TotalIn)), humanize.Bytes(uint64(s.TotalOut)), humanize.Bytes(uint64(s.RateIn)), humanize.Bytes(uint64(s.RateOut)))
			}
		} else if byproto {
			bw, err := netAPI.NetBandwidthStatsByProtocol(ctx)
			if err != nil {
				return err
			}

			var protos []protocol.ID
			for p := range bw {
				protos = append(protos, p)
			}

			sort.Slice(protos, func(i, j int) bool {
				return protos[i] < protos[j]
			})

			for _, p := range protos {
				s := bw[p]
				if p == "" {
					p = "<unknown>"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s/s\t%s/s\n", p, humanize.Bytes(uint64(s.TotalIn)), humanize.Bytes(uint64(s.TotalOut)), humanize.Bytes(uint64(s.RateIn)), humanize.Bytes(uint64(s.RateOut)))
			}
		} else {

			s, err := netAPI.NetBandwidthStats(ctx)
			if err != nil {
				return err
			}

			fmt.Fprintf(tw, "Total\t%s\t%s\t%s/s\t%s/s\n", humanize.Bytes(uint64(s.TotalIn)), humanize.Bytes(uint64(s.TotalOut)), humanize.Bytes(uint64(s.RateIn)), humanize.Bytes(uint64(s.RateOut)))
		}

		if err := tw.Flush(); err != nil {
			return err
		}

		return re.Emit(buf)
	},
}

var idCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show info about the network peers",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addrs, err := env.(*node.Env).NetworkAPI.NetAddrsListen(req.Context)
		if err != nil {
			return err
		}

		hostID := addrs.ID
		details := IDDetails{
			Addresses: make([]ma.Multiaddr, len(addrs.Addrs)),
			ID:        hostID,
		}

		for i, addr := range addrs.Addrs {
			subAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", hostID.Pretty()))
			if err != nil {
				return err
			}
			details.Addresses[i] = addr.Encapsulate(subAddr)
		}

		return re.Emit(&details)
	},
	Type: &IDDetails{},
}

var disconnectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Disconnect from a peer",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peers", true, true, "The peers to disconnect for"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) == 0 {
			return fmt.Errorf("must pass peer id")
		}
		ctx := req.Context

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		ids := req.Arguments
		for _, id := range ids {
			pid, err := peer.Decode(id)
			if err != nil {
				return err
			}
			writer.Printf("disconnect %s", pid.Pretty())
			err = env.(*node.Env).NetworkAPI.NetDisconnect(ctx, pid)
			if err != nil {
				return err
			}
			writer.Println(" success")
		}
		return re.Emit(buf)
	},
}

var reachabilityCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print information about reachability from the internet",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context

		i, err := env.(*node.Env).NetworkAPI.NetAutoNatStatus(ctx)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		writer.Println("AutoNAT status: ", i.Reachability.String())
		if i.PublicAddr != "" {
			writer.Println("Public address: ", i.PublicAddr)
		}

		return re.Emit(buf)
	},
}

var protectAddCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add one or more peer IDs to the list of protected peer connections",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peers", true, true, "need protect peers"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context
		netAPI := env.(*node.Env).NetworkAPI

		pids, err := decodePeerIDsFromArgs(req)
		if err != nil {
			return err
		}

		err = netAPI.NetProtectAdd(ctx, pids)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		writer.Println("added to protected peers:")
		for _, pid := range pids {
			writer.Printf(" %s\n", pid)
		}
		return re.Emit(buf)
	},
}

var protectRemoveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove one or more peer IDs from the list of protected peer connections.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peers", true, true, "need unprotect peers"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context
		netAPI := env.(*node.Env).NetworkAPI

		pids, err := decodePeerIDsFromArgs(req)
		if err != nil {
			return err
		}

		err = netAPI.NetProtectRemove(ctx, pids)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		writer.Println("removed from protected peers:")
		for _, pid := range pids {
			writer.Printf(" %s\n", pid)
		}
		return re.Emit(buf)
	},
}

// decodePeerIDsFromArgs decodes all the arguments present in cli.Context.Args as peer.ID.
//
// This function requires at least one argument to be present, and arguments must not be empty
// string. Otherwise, an error is returned.
func decodePeerIDsFromArgs(req *cmds.Request) ([]peer.ID, error) {
	pidArgs := req.Arguments
	if len(pidArgs) == 0 {
		return nil, fmt.Errorf("must specify at least one peer ID as an argument")
	}
	var pids []peer.ID
	for _, pidStr := range pidArgs {
		if pidStr == "" {
			return nil, fmt.Errorf("peer ID must not be empty")
		}
		pid, err := peer.Decode(pidStr)
		if err != nil {
			return nil, err
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

var protectListCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List the peer IDs with protected connection.",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context
		netAPI := env.(*node.Env).NetworkAPI

		pids, err := netAPI.NetProtectList(ctx)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		for _, pid := range pids {
			writer.Printf("%s\n", pid)
		}

		return re.Emit(buf)
	},
}

// IDDetails is a collection of information about a node.
type IDDetails struct {
	Addresses       []ma.Multiaddr
	ID              peer.ID
	AgentVersion    string
	ProtocolVersion string
	PublicKey       []byte // raw bytes
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
	if idd.ID, err = peer.Decode(id); err != nil {
		return err
	}

	if err := decode(v, "AgentVersion", &idd.AgentVersion); err != nil {
		return err
	}
	if err := decode(v, "ProtocolVersion", &idd.ProtocolVersion); err != nil {
		return err
	}
	return decode(v, "PublicKey", &idd.PublicKey)
}

func decode(idd map[string]*json.RawMessage, key string, dest interface{}) error {
	if raw := idd[key]; raw != nil {
		if err := json.Unmarshal(*raw, &dest); err != nil {
			return err
		}
	}
	return nil
}
