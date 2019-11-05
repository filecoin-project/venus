package fast

import (
	"context"
	"fmt"

	"github.com/ipfs/go-ipfs-files"

	"github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// MiningOnce runs the `mining once` command against the filecoin process
func (f *Filecoin) MiningOnce(ctx context.Context) (*block.Block, error) {
	var out block.Block

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, "go-filecoin", "mining", "once"); err != nil {
		return nil, err
	}

	return &out, nil
}

// MiningSetup prepares the node to receive storage deals
func (f *Filecoin) MiningSetup(ctx context.Context) error {
	out, err := f.RunCmdWithStdin(ctx, nil, "go-filecoin", "mining", "setup")
	if err != nil {
		return err
	}

	if out.ExitCode() > 0 {
		return fmt.Errorf("filecoin command: %s, exited with non-zero exitcode: %d", out.Args(), out.ExitCode())
	}

	return nil
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

// AddPiece runs the mining add-piece command
func (f *Filecoin) AddPiece(ctx context.Context, data files.File) (commands.MiningAddPieceResult, error) {
	var out commands.MiningAddPieceResult
	if err := f.RunCmdJSONWithStdin(ctx, data, &out, "go-filecoin", "mining", "add-piece"); err != nil {
		return out, err
	}
	return out, nil
}

// SealNow seals any staged sectors
func (f *Filecoin) SealNow(ctx context.Context) error {
	out, err := f.RunCmdWithStdin(ctx, nil, "go-filecoin", "mining", "seal-now")
	if err != nil {
		return err
	}

	if out.ExitCode() > 0 {
		return fmt.Errorf("filecoin command: %s, exited with non-zero exitcode: %d", out.Args(), out.ExitCode())
	}

	return nil
}
