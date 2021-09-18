package fast

import (
	"context"
	"github.com/filecoin-project/venus/cmd"
	"strconv"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	cid "github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/pkg/types/specactors/builtin"
)

// MessageSend runs the `message send` command against the filecoin process.
func (f *Filecoin) MessageSend(ctx context.Context, target address.Address, method abi.MethodNum, options ...ActionOption) (cid.Cid, error) {
	var out cmd.MessageSendResult

	args := []string{"venus", "send"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, target.String())

	if method != builtin.MethodSend {
		args = append(args, strconv.Itoa(int(method)))
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return cid.Undef, err
	}

	return out.Cid, nil
}

// MessageWait runs the `message wait` command against the filecoin process.
func (f *Filecoin) MessageWait(ctx context.Context, mcid cid.Cid, options ...ActionOption) (cmd.WaitResult, error) {
	var out cmd.WaitResult

	args := []string{"venus", "message", "wait"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, mcid.String())

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return cmd.WaitResult{}, err
	}

	return out, nil
}
