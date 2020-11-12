package fast

import (
	"context"

	"github.com/filecoin-project/venus/cmd/go-filecoin"
	"github.com/filecoin-project/venus/internal/pkg/config"
)

// InspectAll runs the `inspect all` command against the filecoin process
func (f *Filecoin) InspectAll(ctx context.Context, options ...ActionOption) (*commands.AllInspectorInfo, error) {
	var out commands.AllInspectorInfo

	args := []string{"venus", "inspect", "all"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
}

// InspectRuntime runs the `inspect runtime` command against the filecoin process
func (f *Filecoin) InspectRuntime(ctx context.Context, options ...ActionOption) (*commands.RuntimeInfo, error) {
	var out commands.RuntimeInfo

	args := []string{"venus", "inspect", "runtime"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
}

// InspectDisk runs the `inspect disk` command against the filecoin process
func (f *Filecoin) InspectDisk(ctx context.Context, options ...ActionOption) (*commands.DiskInfo, error) {
	var out commands.DiskInfo

	args := []string{"venus", "inspect", "disk"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
}

// InspectMemory runs the `inspect memory` command against the filecoin process
func (f *Filecoin) InspectMemory(ctx context.Context, options ...ActionOption) (*commands.MemoryInfo, error) {
	var out commands.MemoryInfo

	args := []string{"venus", "inspect", "memory"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
}

// InspectConfig runs the `inspect config` command against the filecoin process
func (f *Filecoin) InspectConfig(ctx context.Context, options ...ActionOption) (*config.Config, error) {
	var out config.Config

	args := []string{"venus", "inspect", "config"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
}
