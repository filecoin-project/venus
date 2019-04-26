package fast

import (
	"context"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/plumbing/inspector"
)

// InspectRuntime runs the `inspect runtime` command against the filecoin process
func (f *Filecoin) InspectRuntime(ctx context.Context, options ...ActionOption) (*inspector.RuntimeInfo, error) {
	var out inspector.RuntimeInfo

	args := []string{"go-filecoin", "inspect", "runtime"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
}

// InspectDisk runs the `inspect disk` command against the filecoin process
func (f *Filecoin) InspectDisk(ctx context.Context, options ...ActionOption) (*inspector.DiskInfo, error) {
	var out inspector.DiskInfo

	args := []string{"go-filecoin", "inspect", "disk"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
}

// InspectMemory runs the `inspect memory` command against the filecoin process
func (f *Filecoin) InspectMemory(ctx context.Context, options ...ActionOption) (*inspector.MemoryInfo, error) {
	var out inspector.MemoryInfo

	args := []string{"go-filecoin", "inspect", "memory"}

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

	args := []string{"go-filecoin", "inspect", "config"}

	for _, option := range options {
		args = append(args, option()...)
	}

	if err := f.RunCmdJSONWithStdin(ctx, nil, &out, args...); err != nil {
		return nil, err
	}

	return &out, nil
}
