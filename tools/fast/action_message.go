package fast

import (
	"context"

	cid "github.com/ipfs/go-cid"

	commands "github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// MessageSend runs the `message send` command against the filecoin process.
func (f *Filecoin) MessageSend(ctx context.Context, target address.Address, method types.MethodID, options ...ActionOption) (cid.Cid, error) {
	var out commands.MessageSendResult

	args := []string{"go-filecoin", "message", "send"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, target.String())

	if method != types.SendMethodID {
		args = append(args, method.String())
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return cid.Undef, err
	}

	return out.Cid, nil
}

// MessageWait runs the `message wait` command against the filecoin process.
func (f *Filecoin) MessageWait(ctx context.Context, mcid cid.Cid, options ...ActionOption) (commands.WaitResult, error) {
	var out commands.WaitResult

	args := []string{"go-filecoin", "message", "wait"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, mcid.String())

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return commands.WaitResult{}, err
	}

	return out, nil
}
