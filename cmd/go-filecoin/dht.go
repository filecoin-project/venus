package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
)

const (
	dhtVerboseOptionName   = "verbose"
	numProvidersOptionName = "num-providers"
)

// Note, most of this is copied directly from go-ipfs (https://github.com/ipfs/go-ipfs/blob/master/core/commands/dht.go).
// A few simple modifications have been adapted for filecoin.
var dhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Explore and manipulate the libp2p DHT.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"findprovs": findProvidersDhtCmd,
		"findpeer":  findPeerDhtCmd,
		"query":     queryDhtCmd,
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

		closestPeers, err := GetPorcelainAPI(env).NetworkGetClosestPeers(ctx, string(id))
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

		pchan := GetPorcelainAPI(env).NetworkFindProvidersAsync(ctx, c, numProviders)

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

		out, err := GetPorcelainAPI(env).NetworkFindPeer(req.Context, peerID)
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
