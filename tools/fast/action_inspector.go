package fast

import (
	"context"
	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/config"
)

// InspectAll runs the `inspect all` command against the filecoin process
func (f *Filecoin) InspectAll(ctx context.Context, options ...ActionOption) (*node.AllInspectorInfo, error) {
	var out node.AllInspectorInfo

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
func (f *Filecoin) InspectRuntime(ctx context.Context, options ...ActionOption) (*node.RuntimeInfo, error) {
	var out node.RuntimeInfo

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
func (f *Filecoin) InspectDisk(ctx context.Context, options ...ActionOption) (*node.DiskInfo, error) {
	var out node.DiskInfo

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
func (f *Filecoin) InspectMemory(ctx context.Context, options ...ActionOption) (*node.MemoryInfo, error) {
	var out node.MemoryInfo

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
