package fast

import (
	"context"
	"encoding/json"
	"fmt"
)

// ConfigGet runs the `config` command against the filecoin process, and decodes the
// output into `v`.
func (f *Filecoin) ConfigGet(ctx context.Context, key string, v interface{}) error {
	args := []string{"venus", "config", key}

	if err := f.RunCmdJSONWithStdin(ctx, nil, v, args...); err != nil {
		return err
	}

	return nil
}

// ConfigSet runs the `config` command against the filecoin process, encoding `v` as
// the value.
func (f *Filecoin) ConfigSet(ctx context.Context, key string, v interface{}) error {
	value, err := json.Marshal(v)
	if err != nil {
		return err
	}

	args := []string{"venus", "config", key, string(value)}

	out, err := f.RunCmdWithStdin(ctx, nil, args...)
	if err != nil {
		return err
	}

	// check command exit code
	if out.ExitCode() > 0 {
		return fmt.Errorf("filecoin command: %s, exited with non-zero exitcode: %d", out.Args(), out.ExitCode())
	}

	return nil
}
