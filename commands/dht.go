package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	pstore "gx/ipfs/QmRhFARzTHcFh8wUxwN5KvyTGq73FLC65EfFAhz8Ng7aGb/go-libp2p-peerstore"
	notif "gx/ipfs/QmWaDSNoSdSXU9b6udyaq9T8y6LkzMwqWxECznFqvtcTsk/go-libp2p-routing/notifications"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	cmds "gx/ipfs/Qmf46mr235gtyxizkKUkTH5fo62Thza2zwXR4DWC7rkoqF/go-ipfs-cmds"
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
	},
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
		ctx, events := notif.RegisterForQueryEvents(ctx)

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
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:      notif.Provider,
					Responses: []*pstore.PeerInfo{&np},
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
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *notif.QueryEvent) error {
			pfm := pfuncMap{
				notif.FinalPeer: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					if verbose {
						fmt.Fprintf(out, "* closest peer %s\n", obj.ID) // nolint: errcheck
					}
				},
				notif.Provider: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
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
	Type: notif.QueryEvent{},
}

type printFunc func(obj *notif.QueryEvent, out io.Writer, verbose bool)
type pfuncMap map[notif.QueryEventType]printFunc

// printEvent writes a libp2p event to a user friendly output on the out writer.
// Note that this function is only needed to enable the output logging of
// events that only show up during a "verbose" run. If we choose to eliminate
// the verbose option this can be removed. However if we keep the verbose option
// in findprovs and on any other dht subcommands we decide to copy over from
// ipfs this function will stay needed.
func printEvent(obj *notif.QueryEvent, out io.Writer, verbose bool, override pfuncMap) {
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
	case notif.SendingQuery:
		if verbose {
			fmt.Fprintf(out, "* querying %s\n", obj.ID) // nolint: errcheck
		}
	case notif.Value:
		if verbose {
			fmt.Fprintf(out, "got value: '%s'\n", obj.Extra) // nolint: errcheck
		} else {
			fmt.Fprint(out, obj.Extra) // nolint: errcheck
		}
	case notif.PeerResponse:
		if verbose {
			fmt.Fprintf(out, "* %s says use ", obj.ID) // nolint: errcheck
			for _, p := range obj.Responses {
				fmt.Fprintf(out, "%s ", p.ID) // nolint: errcheck
			}
			fmt.Fprintln(out) // nolint: errcheck
		}
	case notif.QueryError:
		if verbose {
			fmt.Fprintf(out, "error: %s\n", obj.Extra) // nolint: errcheck
		}
	case notif.DialingPeer:
		if verbose {
			fmt.Fprintf(out, "dialing peer: %s\n", obj.ID) // nolint: errcheck
		}
	case notif.AddingPeer:
		if verbose {
			fmt.Fprintf(out, "adding peer to query: %s\n", obj.ID) // nolint: errcheck
		}
	case notif.FinalPeer:
	default:
		if verbose {
			fmt.Fprintf(out, "unrecognized event type: %d\n", obj.Type) // nolint: errcheck
		}
	}
}
