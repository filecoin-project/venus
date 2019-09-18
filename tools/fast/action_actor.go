package fast

import (
	"context"
	"io"

	"github.com/filecoin-project/go-filecoin/commands"
	"github.com/filecoin-project/go-filecoin/state"
)

// ActorLs runs the `actor ls` command against the filecoin process.
func (f *Filecoin) ActorLs(ctx context.Context) ([]commands.ActorView, error) {
	args := []string{"go-filecoin", "actor", "ls"}

	dec, err := f.RunCmdLDJSONWithStdin(ctx, nil, args...)
	if err != nil {
		return nil, err
	}

	views := []commands.ActorView{}
	for dec.More() {
		var view commands.ActorView
		err := dec.Decode(&view)
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		views = append(views, view)
	}

	return views, nil
}

// ActorPower runs the `actor power-table` command against the filecoin process.
func (f *Filecoin) ActorPower(ctx context.Context) ([]state.PowerTable, error) {
	args := []string{"go-filecoin", "actor", "power-table"}

	dec, err := f.RunCmdLDJSONWithStdin(ctx, nil, args...)
	if err != nil {
		return nil, err
	}

	tables := []state.PowerTable{}
	for dec.More() {
		var table state.PowerTable
		err := dec.Decode(&table)
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		tables = append(tables, table)
	}

	return tables, nil
}
