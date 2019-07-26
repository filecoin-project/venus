package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
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
	Helptext: cmdkit.HelpText{
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
	Helptext: cmdkit.HelpText{
		Tagline:          "Find the closest Peer IDs to a given Peer ID by querying the DHT.",
		ShortDescription: "Outputs a list of newline-delimited Peer IDs.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peerID", true, false, "The peerID to run the query against."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(dhtVerboseOptionName, "v", "Print extra information."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {

		id, err := peer.IDB58Decode(req.Arguments[0])
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
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *routing.QueryEvent) error {
			pfm := pfuncMap{
				routing.PeerResponse: func(obj *routing.QueryEvent, out io.Writer, verbose bool) {
					for _, p := range obj.Responses {
						fmt.Fprintf(out, "%s\n", p.ID.Pretty()) // nolint: errcheck
					}
				},
			}
			verbose, _ := req.Options[dhtVerboseOptionName].(bool)
			printEvent(out, w, verbose, pfm)
			return nil
		}),
	},
	Type: routing.QueryEvent{},
}

var findProvidersDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Find peers that can provide a given key's value.",
		ShortDescription: "Outputs a list of newline-delimited provider Peer IDs for a given key.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, false, "The key whose provider Peer IDs are output.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(dhtVerboseOptionName, "v", "Print extra information."),
		cmdkit.IntOption(numProvidersOptionName, "n", "The max number of providers to find.").WithDefault(20),
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
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *routing.QueryEvent) error {
			pfm := pfuncMap{
				routing.FinalPeer: func(obj *routing.QueryEvent, out io.Writer, verbose bool) {
					if verbose {
						fmt.Fprintf(out, "* closest peer %s\n", obj.ID) // nolint: errcheck
					}
				},
				routing.Provider: func(obj *routing.QueryEvent, out io.Writer, verbose bool) {
					prov := obj.Responses[0]
					if verbose {
						fmt.Fprintf(out, "provider: ") // nolint: errcheck
					}
					fmt.Fprintf(out, "%s\n", prov.ID.Pretty()) // nolint: errcheck
					if verbose {
						for _, a := range prov.Addrs {
							fmt.Fprintf(out, "\t%s\n", a) // nolint: errcheck
						}
					}
				},
			}

			verbose, _ := req.Options[dhtVerboseOptionName].(bool)
			printEvent(out, w, verbose, pfm)

			return nil
		}),
	},
	Type: routing.QueryEvent{},
}

type printFunc func(obj *routing.QueryEvent, out io.Writer, verbose bool)
type pfuncMap map[routing.QueryEventType]printFunc

// printEvent writes a libp2p event to a user friendly output on the out writer.
// Note that this function is only needed to enable the output logging of
// events that only show up during a "verbose" run. If we choose to eliminate
// the verbose option this can be removed. However if we keep the verbose option
// in findprovs and on any other dht subcommands we decide to copy over from
// ipfs this function will stay needed.
func printEvent(obj *routing.QueryEvent, out io.Writer, verbose bool, override pfuncMap) {
	if verbose {
		fmt.Fprintf(out, "%s: ", time.Now().Format("15:04:05.000")) // nolint: errcheck
	}

	if override != nil {
		if pf, ok := override[obj.Type]; ok {
			pf(obj, out, verbose)
			return
		}
	}

	switch obj.Type {
	case routing.SendingQuery:
		if verbose {
			fmt.Fprintf(out, "* querying %s\n", obj.ID) // nolint: errcheck
		}
	case routing.Value:
		if verbose {
			fmt.Fprintf(out, "got value: '%s'\n", obj.Extra) // nolint: errcheck
		} else {
			fmt.Fprint(out, obj.Extra) // nolint: errcheck
		}
	case routing.PeerResponse:
		if verbose {
			fmt.Fprintf(out, "* %s says use ", obj.ID) // nolint: errcheck
			for _, p := range obj.Responses {
				fmt.Fprintf(out, "%s ", p.ID) // nolint: errcheck
			}
			fmt.Fprintln(out) // nolint: errcheck
		}
	case routing.QueryError:
		if verbose {
			fmt.Fprintf(out, "error: %s\n", obj.Extra) // nolint: errcheck
		}
	case routing.DialingPeer:
		if verbose {
			fmt.Fprintf(out, "dialing peer: %s\n", obj.ID) // nolint: errcheck
		}
	case routing.AddingPeer:
		if verbose {
			fmt.Fprintf(out, "adding peer to query: %s\n", obj.ID) // nolint: errcheck
		}
	case routing.FinalPeer:
	default:
		if verbose {
			fmt.Fprintf(out, "unrecognized event type: %d\n", obj.Type) // nolint: errcheck
		}
	}
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
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, addr string) error {
			_, err := fmt.Fprintln(w, addr)
			return err
		}),
	},
}
