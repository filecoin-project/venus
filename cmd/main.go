package cmd

import (
	"context"
	"errors"
	"net"
	"os"

	cmds "gx/ipfs/Qmc5paX4ECBARnAKkcAmUYHBGor228Tkfxeya3Nu2KRL46/go-ipfs-cmds"
	cmdcli "gx/ipfs/Qmc5paX4ECBARnAKkcAmUYHBGor228Tkfxeya3Nu2KRL46/go-ipfs-cmds/cli"
	cmdhttp "gx/ipfs/Qmc5paX4ECBARnAKkcAmUYHBGor228Tkfxeya3Nu2KRL46/go-ipfs-cmds/http"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

const (
	// OptionAPI is the name of the option for specifying the api port.
	OptionAPI = "cmdapiaddr"
	// APIPrefix is the prefix for the http version of the api.
	APIPrefix = "/api"
)

var (
	// ErrAlreadyRunning is the error returned when trying to start the daemon, even though it is already running.
	ErrAlreadyRunning = errors.New("daemon is already running")
	// ErrMissingDaemon is the error returned when trying to execute a command that requires the daemon to be started.
	ErrMissingDaemon = errors.New("daemon must be started before using this command")
)

var rootCmd = &cmds.Command{
	Options: []cmdkit.Option{
		cmdkit.StringOption(OptionAPI, "set the api port to use").WithDefault(":3453"),
		cmds.OptionEncodingType,
	},
	Subcommands: make(map[string]*cmds.Command),
}

// All commands that require the daemon to be running
var rootSubcmdsDaemon = map[string]*cmds.Command{}

// All commands that require the daemon _not_ to be running
var rootSubcmdsNoDaemon = map[string]*cmds.Command{
	"daemon":  daemonCmd,
	"version": versionCmd,
}

func init() {
	for k, v := range rootSubcmdsDaemon {
		rootCmd.Subcommands[k] = v
	}

	for k, v := range rootSubcmdsNoDaemon {
		rootCmd.Subcommands[k] = v
	}
}

// Run processes the arguments and stdin
func Run(args []string, stdin *os.File) (int, error) {
	req, err := cmdcli.Parse(context.Background(), args[1:], stdin, rootCmd)
	if err != nil {
		return 1, err
	}

	api := req.Options[OptionAPI].(string)
	isDaemonRunning, err := daemonRunning(api)
	if err != nil {
		return 1, err
	}

	if isDaemonRunning && req.Command == daemonCmd {
		return 1, ErrAlreadyRunning
	}

	if !isDaemonRunning && requiresDaemon(req) {
		return 1, ErrMissingDaemon
	}

	if isDaemonRunning {
		return dispatchRemoteCmd(req, api)
	}

	return dispatchLocalCmd(req)
}

func requiresDaemon(req *cmds.Request) bool {
	for _, v := range rootSubcmdsDaemon {
		if req.Command == v {
			return true
		}
	}
	return false
}

func dispatchRemoteCmd(req *cmds.Request, api string) (int, error) {
	client := cmdhttp.NewClient(api, cmdhttp.ClientWithAPIPrefix(APIPrefix))

	// send request to server
	res, err := client.Send(req)
	if err != nil {
		return 1, err
	}

	encType := cmds.GetEncoding(req)
	enc := req.Command.Encoders[encType]
	if enc == nil {
		enc = cmds.Encoders[encType]
	}

	// create an emitter
	re, retCh := cmdcli.NewResponseEmitter(os.Stdout, os.Stderr, enc, req)

	if pr, ok := req.Command.PostRun[cmds.CLI]; ok {
		re = pr(req, re)
	}

	wait := make(chan struct{})
	// copy received result into cli emitter
	go func() {
		err = cmds.Copy(re, res)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal|cmdkit.ErrFatal)
		}
		close(wait)
	}()

	// wait until command has returned and exit
	ret := <-retCh
	<-wait

	return ret, nil
}

func dispatchLocalCmd(req *cmds.Request) (ret int, err error) {
	req.Options[cmds.EncLong] = cmds.Text

	// create an emitter
	re, retCh := cmdcli.NewResponseEmitter(os.Stdout, os.Stderr, req.Command.Encoders[cmds.Text], req)

	if pr, ok := req.Command.PostRun[cmds.CLI]; ok {
		re = pr(req, re)
	}

	wait := make(chan struct{})
	// call command in background
	go func() {
		defer close(wait)

		err = rootCmd.Call(req, re, nil)
		if err != nil {
			panic(err)
		}
	}()

	// wait until command has returned and exit
	ret = <-retCh
	<-wait

	return ret, err
}

func daemonRunning(api string) (bool, error) {
	// TODO: use lockfile once implemented
	// for now we just check if the port is available

	ln, err := net.Listen("tcp", api)
	if err != nil {
		return true, nil
	}

	if err := ln.Close(); err != nil {
		return false, err
	}

	return false, nil
}
