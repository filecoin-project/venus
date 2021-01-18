package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/filecoin-project/venus/app/node"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	ma "github.com/multiformats/go-multiaddr"
	"time"

	"github.com/filecoin-project/venus/pkg/net"
)

const (
	dhtVerboseOptionName   = "verbose"
	numProvidersOptionName = "num-providers"
)

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
		"id":        idCmd,
		"query":     queryDhtCmd,
		"peers":     swarmPeersCmd,
		"connect":   swarmConnectCmd,
		"findpeer":  findPeerDhtCmd,
		"findprovs": findProvidersDhtCmd,
		"bandwidth": statsBandwidthCmd,
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
		cmds.BoolOption("verbose", "v", "Display all extra information"),
		cmds.BoolOption("streams", "Also list information about open streams for each peer"),
		cmds.BoolOption("latency", "Also list information about latency to each peer"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		verbose, _ := req.Options["verbose"].(bool)
		latency, _ := req.Options["latency"].(bool)
		streams, _ := req.Options["streams"].(bool)

		out, err := env.(*node.Env).NetworkAPI.NetworkPeers(req.Context, verbose, latency, streams)
		if err != nil {
			return err
		}

		return re.Emit(&out)
	},
	Type: net.SwarmConnInfos{},
}

var swarmConnectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Open connection to a given address.",
		ShortDescription: `
'venus swarm connect' opens a new direct connection to a peer address.

The address format is a multiaddr:

venus swarm connect /ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("address", true, true, "address of peer to connect to.").EnableStdin(),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		results, err := env.(*node.Env).NetworkAPI.NetworkConnect(req.Context, req.Arguments)
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

		closestPeers, err := env.(*node.Env).NetworkAPI.NetworkGetClosestPeers(ctx, string(id))
		if err != nil {
			cancel()
			return err
		}

		go func() {
			defer cancel()
			for p := range closestPeers {
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

		pchan := env.(*node.Env).NetworkAPI.NetworkFindProvidersAsync(ctx, c, numProviders)

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

		out, err := env.(*node.Env).NetworkAPI.NetworkFindPeer(req.Context, peerID)
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		bandwidthStats := env.(*node.Env).NetworkAPI.NetworkGetBandwidthStats()
		return re.Emit(bandwidthStats)
	},
	Type: metrics.Stats{},
}

// IDDetails is a collection of information about a node.
type IDDetails struct {
	Addresses       []ma.Multiaddr
	ID              peer.ID
	AgentVersion    string
	ProtocolVersion string
	PublicKey       []byte // raw bytes
}

var idCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show info about the network peers",
	},
	Options: []cmds.Option{
		// TODO: ideally copy this from the `ipfs id` command
		cmds.StringOption("format", "f", "Specify an output format"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addrs := env.(*node.Env).NetworkAPI.NetworkGetPeerAddresses()
		hostID := env.(*node.Env).NetworkAPI.NetworkGetPeerID()

		details := IDDetails{
			Addresses: make([]ma.Multiaddr, len(addrs)),
			ID:        hostID,
		}

		for i, addr := range addrs {
			subAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", hostID.Pretty()))
			if err != nil {
				return err
			}
			details.Addresses[i] = addr.Encapsulate(subAddr)
		}

		return re.Emit(&details)
	},
	Type: IDDetails{},
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
