package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	cmdsCli "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds/cli"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

// Closer is a helper interface to check if the env supports closing
type Closer interface {
	Close()
}

// CliRun is copied and modified from go-ipfs-commands/cli/run.go
// TODO: remove once upstream is updated
func CliRun(ctx context.Context, root *cmds.Command,
	cmdline []string, stdin, stdout, stderr *os.File,
	buildEnv cmds.MakeEnvironment, makeExecutor cmds.MakeExecutor) (int, error) {

	printErr := func(err error) {
		fmt.Fprintf(stderr, "%s\n", err)
	}

	req, errParse := cmdsCli.Parse(ctx, cmdline[1:], stdin, root)

	// Handle the timeout up front.
	var cancel func()
	if timeoutStr, ok := req.Options[cmds.TimeoutOpt]; ok {
		timeout, err := time.ParseDuration(timeoutStr.(string))
		if err != nil {
			return 1, err
		}
		req.Context, cancel = context.WithTimeout(req.Context, timeout)
	} else {
		req.Context, cancel = context.WithCancel(req.Context)
	}
	defer cancel()

	// this is a message to tell the user how to get the help text
	printMetaHelp := func(w io.Writer) {
		cmdPath := strings.Join(req.Path, " ")
		fmt.Fprintf(w, "Use '%s %s --help' for information about this command\n", cmdline[0], cmdPath)
	}

	printHelp := func(long bool, w io.Writer) {
		helpFunc := cmdsCli.ShortHelp
		if long {
			helpFunc = cmdsCli.LongHelp
		}

		var path []string
		if req != nil {
			path = req.Path
		}

		if err := helpFunc(cmdline[0], root, path, w); err != nil {
			// This should not happen
			panic(err)
		}
	}

	// BEFORE handling the parse error, if we have enough information
	// AND the user requested help, print it out and exit
	err := cmdsCli.HandleHelp(cmdline[0], req, stdout)
	if err == nil {
		return 0, nil
	} else if err != cmdsCli.ErrNoHelpRequested {
		return 1, err
	}
	// no help requested, continue.

	// ok now handle parse error (which means cli input was wrong,
	// e.g. incorrect number of args, or nonexistent subcommand)
	if errParse != nil {
		printErr(errParse)

		// this was a user error, print help
		if req != nil && req.Command != nil {
			fmt.Fprintln(stderr) // i need some space
			printHelp(false, stderr)
		}

		return 1, err
	}

	// here we handle the cases where
	// - commands with no Run func are invoked directly.
	// - the main command is invoked.
	if req == nil || req.Command == nil || req.Command.Run == nil {
		printHelp(false, stdout)
		return 0, nil
	}

	cmd := req.Command

	env, err := buildEnv(req.Context, req)
	if err != nil {
		printErr(err)
		return 1, err
	}
	if c, ok := env.(Closer); ok {
		defer c.Close()
	}

	exctr, err := makeExecutor(req, env)
	if err != nil {
		printErr(err)
		return 1, err
	}

	var (
		re     cmds.ResponseEmitter
		exitCh <-chan int
	)

	encTypeStr, _ := req.Options[cmds.EncLong].(string)
	encType := cmds.EncodingType(encTypeStr)

	// use JSON if text was requested but the command doesn't have a text-encoder
	if _, ok := cmd.Encoders[encType]; encType == cmds.Text && !ok {
		req.Options[cmds.EncLong] = cmds.JSON
	}

	// first if condition checks the command's encoder map, second checks global encoder map (cmd vs. cmds)
	if enc, ok := cmd.Encoders[encType]; ok {
		re, exitCh = cmdsCli.NewResponseEmitter(stdout, stderr, enc, req)
	} else if enc, ok := cmds.Encoders[encType]; ok {
		re, exitCh = cmdsCli.NewResponseEmitter(stdout, stderr, enc, req)
	} else {
		return 1, fmt.Errorf("could not find matching encoder for enctype %#v", encType)
	}

	errCh := make(chan error, 1)
	go func() {
		err := exctr.Execute(req, re, env)
		if err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		printErr(err)

		if kiterr, ok := err.(*cmdkit.Error); ok {
			err = *kiterr
		}
		if kiterr, ok := err.(cmdkit.Error); ok && kiterr.Code == cmdkit.ErrClient {
			printMetaHelp(stderr)
		}

		return 1, err

	case code := <-exitCh:
		if code != 0 {
			return code, nil
		}
	}

	return 0, nil
}
