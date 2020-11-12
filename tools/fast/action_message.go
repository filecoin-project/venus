package fast

import (
	"context"
	"strconv"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	cid "github.com/ipfs/go-cid"

	commands "github.com/filecoin-project/venus/cmd/go-filecoin"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin"
)

// MessageSend runs the `message send` command against the filecoin process.
func (f *Filecoin) MessageSend(ctx context.Context, target address.Address, method abi.MethodNum, options ...ActionOption) (cid.Cid, error) {
	var out commands.MessageSendResult

	args := []string{"venus", "message", "send"}

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
func (f *Filecoin) MessageWait(ctx context.Context, mcid cid.Cid, options ...ActionOption) (commands.WaitResult, error) {
	var out commands.WaitResult

	args := []string{"venus", "message", "wait"}

	for _, option := range options {
		args = append(args, option()...)
	}

	args = append(args, mcid.String())

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return commands.WaitResult{}, err
	}

	return out, nil
}
