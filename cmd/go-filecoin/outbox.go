package commands

import (
	"io"

	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

var outboxCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "View and manipulate the outbound message queue",
	},
	Subcommands: map[string]*cmds.Command{
		"clear": outboxClearCmd,
		"ls":    outboxLsCmd,
	},
}

// OutboxLsResult is a listing of the outbox for a single address.
type OutboxLsResult struct {
	Address  address.Address
	Messages []*message.Queued
}

var outboxLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List the queue(s) of sent but un-mined messages",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", false, false, "Address of the queue to list (otherwise lists all)"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addresses, err := queueAddressesFromArg(req, env, 0)
		if err != nil {
			return err
		}

		for _, addr := range addresses {
			msgs := GetPorcelainAPI(env).OutboxQueueLs(addr)
			err := re.Emit(OutboxLsResult{addr, msgs})
			if err != nil {
				return err
			}
		}
		return nil
	},
	Type: OutboxLsResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, queue *OutboxLsResult) error {
			sw := NewSilentWriter(w)
			sw.Println("From:", queue.Address.String())
			for _, qm := range queue.Messages {
				msg := qm.Msg
				sw.Printf("%s, height: %d\n", msg.String(), qm.Stamp)
			}
			return sw.Error()
		}),
	},
}

var outboxClearCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Clear the queue(s) of sent messages",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("address", false, false, "Address of the queue to clear (otherwise clears all)"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		addresses, err := queueAddressesFromArg(req, env, 0)
		if err != nil {
			return err
		}

		for _, addr := range addresses {
			GetPorcelainAPI(env).OutboxQueueClear(req.Context, addr)
		}
		return nil
	},
	Encoders: cmds.EncoderMap{},
}

// Reads an address from an argument, or lists addresses of all outbox queues if no arg is given.
func queueAddressesFromArg(req *cmds.Request, env cmds.Environment, argIndex int) ([]address.Address, error) {
	var addresses []address.Address
	if len(req.Arguments) > argIndex {
		addr, e := address.NewFromString(req.Arguments[argIndex])
		if e != nil {
			return nil, e
		}
		addresses = []address.Address{addr}
	} else {
		addresses = GetPorcelainAPI(env).OutboxQueues()
	}
	return addresses, nil
}
