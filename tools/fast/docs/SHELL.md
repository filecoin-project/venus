### The FAST Shell

FAST has a [`Shell`](https://godoc.org/github.com/filecoin-project/go-filecoin/tools/fast#Filecoin.Shell) which can be used to start the users shell with the environment setup to run `go-filecoin` commands against the process it is called on.

The exact environment the shell will have is largely dependent on the plugin use are using.
Generally, this will be the Filecoin localplugin when writing tests using FAST.

The Filecoin localplugin shell environment will have the following variables set in the shell for the user.

| Name  | Description |
|:---|:---|
| `FIL_PATH` |  The value is set to the repository directory for the Filecoin node. Any `go-filecoin` commands ran in the shell will be executed against the Filecoin process for which the Shell method was invoked on in FAST. |
| `FIL_PID` | The value is set to the PID for the Filecoin daemon. |
| `FIL_BINARY` | The value is set to the binary running the Filecoin daemon. Please refer to the section below on `PATH` for more details. |
| `PATH` | The users `PATH` will be updated to include a location that contains the binary used for executing all `go-filecoin` commands. The `go-filecoin` binary included in this location itself is defined by either the value of the `localplugin.AttrFilecoinBinary`, or the first `go-filecoin` binary found in the users `PATH`. <br/> <br/>_Note: The value of `FIL_BINARY` will not be the exact value. During node setup, the binary is copied to ensure it does not change during execution. `FIL_BINARY` will be this new path._ |

The information around the `PATH` seems a little complex, but it's to ensure that there are no issues as a result of mixing binaries.
This has the advantage that while using FAST, users can re-compile `go-filecoin` without affecting constructed nodes.
It should be noted that the copying of the binary occurs during the call to `NewProcess`.

It should also be noted that users shell configuration will be ran when the shell opens.
If shell configuration updates the `PATH` by appending to the front, if any of those directories contain `go-filecoin`, then the `go-filecoin` command inside of the FAST Shell will **not** point to the correct binary, because of this, it is best to actually execute commands using the `$FIL_BINARY` variable (eg: `$FIL_BINARY id`).

### Using a FAST Shell in _go test_

The FAST Shell should only be used when running a single test.

_Note: When using the FAST Shell, it's best to increase deadlines set for tests, as it's very easy to exceed them, and you will be kicked out of your shell._
```diff
-       ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
+       ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Day))
```

When using FAST inside of go tests, `stdin` is not properly configured to the users shell.
This results in the shell exiting immediately, with no error.
To resolve this issue the Filecoin localplugin will attach the fd defined by the environment variables `TTY` to the shell it opens.
This will allows the user to define which TTY to use for stdin. Generally this should be the same TTY of the shell `go test` is ran in.
This can easily be set by using the `tty` program.

```shell
$ env TTY=$(tty) go test -v -run TestFilecoin
```

Test execution can be paused, much like a breakpoint, by dropping a call to `node.Shell()` anywhere _after_ the daemon has been started.
It is used to wrap the call of in a `require.NoError` which will ensure the test fails quickly if the expected shell has an issue starting.
You can also kill the test by exiting the shell with `exit 1` as the shell will return a non-zero exit code as an error.

```go
require.NoError(node.Shell())
```

While in the shell, no daemons are paused, but further test code execution is paused till the shell exits.

#### FAST Shell use cases

##### Attaching a debugger to the node

The shell environment provides the daemon pid and binary through `FIL_PID` and `FIL_BINARY` respectfully.
A debugger, such as `dlv`, can be attached using these values.

```shell
$ dlv $FIL_PID $FIL_BINARY
```

You won't be able to continue the test if the debugger is attached in the shell, but the values can be easily printed, and the debugger opened in a new shell.
Once you have everything setup and are ready to continue the test, simply exit the shell and the test will continue.

##### Capturing daemon logs produced during shell use.

Tests can be used to get nodes into certain state.
You may want to then use a shell to execute additional commands to debug an issue.
You may have added additional logging you want to look at, or look at existing logging that will be produced by commands you run.

FAST Provides a [`StartLogCapture`](https://godoc.org/github.com/filecoin-project/go-filecoin/tools/fast#Filecoin.StartLogCapture) which will capture all output written to the daemons `stderr` until `Stop` is called.
The captured logs are stored in the return value of `StartLogCapture`, which can be copied to any `io.Writer`.

```go
interval, err := node.StartLogCapture()
require.NoError(err)
require.NoError(node.Shell())
interval.Stop()
io.Copy(os.Stdout, interval)
```
