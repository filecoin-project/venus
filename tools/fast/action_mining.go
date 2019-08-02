package fast

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/commands"
	"github.com/filecoin-project/go-filecoin/types"
)

// MiningOnce runs the `mining once` command against the filecoin process
func (f *Filecoin) MiningOnce(ctx context.Context) (*types.Block, error) {
	var out types.Block

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "mining", "once"); err != nil {
		return nil, err
	}

	return &out, nil
}

// MiningStart runs the `mining Start` command against the filecoin process
func (f *Filecoin) MiningStart(ctx context.Context) error {
	out, err := f.RunCmdWithStdin(ctx, nil, "go-filecoin", "mining", "start")
	if err != nil {
		return err
	}

	if out.ExitCode() > 0 {
		return fmt.Errorf("filecoin command: %s, exited with non-zero exitcode: %d", out.Args(), out.ExitCode())
	}

	return nil
}

// MiningStop runs the `mining stop` command against the filecoin process
func (f *Filecoin) MiningStop(ctx context.Context) error {
	out, err := f.RunCmdWithStdin(ctx, nil, "go-filecoin", "mining", "stop")
	if err != nil {
		return err
	}

	if out.ExitCode() > 0 {
		return fmt.Errorf("filecoin command: %s, exited with non-zero exitcode: %d", out.Args(), out.ExitCode())
	}

	return nil
}

// MiningAddress runs the `mining address` command against the filecoin process
func (f *Filecoin) MiningAddress(ctx context.Context) (address.Address, error) {
	var out address.Address

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "mining", "address"); err != nil {
		return address.Undef, err
	}

	return out, nil
}

// MiningStatus runs the `mining status` command against the filecoin process
func (f *Filecoin) MiningStatus(ctx context.Context) (commands.MiningStatusResult, error) {
	var out commands.MiningStatusResult

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "mining", "status"); err != nil {
		return commands.MiningStatusResult{}, err
	}

	return out, nil
}
