package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	cmdsCli "gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds/cli"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
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
		fmt.Fprintf(stderr, "%s\n", err) // nolint: errcheck
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
		fmt.Fprintf(w, "Use '%s %s --help' for information about this command\n", cmdline[0], cmdPath) // nolint: errcheck
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
			fmt.Fprintln(stderr) // nolint: errcheck
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
		encType = cmds.JSON
	}

	// first if condition checks the command's encoder map, second checks global encoder map (cmd vs. cmds)
	// TODO verify this change wrt EncoderMap leaving
	re, exitCh, err = cmdsCli.NewResponseEmitter(stdout, stderr, req)

	// Run the command and send any errors that it returns to the error
	// channel execErrCh. Commands should have their panic propagated to the
	// main goroutine. The main goroutine must not exit before the child
	// goroutine's panic is propagated or the panic will be lost.

	// buffered channel which may see an error returned from Execute()
	execErrCh := make(chan error, 1)

	// buffered channel which may receive a panic from Execute()
	execPanicCh := make(chan interface{}, 1)

	// used to block main goroutine until child goroutine exits
	execWg := sync.WaitGroup{}
	execWg.Add(1)

	// child goroutine in which we Execute() the command
	go func() {
		defer func() {
			if e := recover(); e != nil {
				fmt.Fprintf(stderr, "%s: %s", e, debug.Stack()) // nolint: errcheck
				execPanicCh <- e
			}
			execWg.Done()
		}()

		// Execute() blocks until we get a value from exitCh. Note that exitCh
		// will see the default exit code, 0, if a non-daemon command panics
		// before calling ResponseEmitter#Emit.
		if err := exctr.Execute(req, re, env); err != nil {
			execErrCh <- err
		}
	}()

	// unblocks the child goroutine
	code := <-exitCh

	// wait for deferred panic-handler
	execWg.Wait()

	select {
	case e := <-execPanicCh:
		panic(e)
	case err := <-execErrCh:
		printErr(err)

		if kiterr, ok := err.(*cmdkit.Error); ok {
			err = *kiterr
		}
		if kiterr, ok := err.(cmdkit.Error); ok && kiterr.Code == cmdkit.ErrClient {
			printMetaHelp(stderr)
		}

		return 1, err
	default:
		return code, nil
	}
}
