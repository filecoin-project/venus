# go-fakecoin

`go-fakecoin` is a command-line tool which can be used to generate well-formed Filecoin data for testing purposes.

# Building

First build Filecoin as usual.

Then, from the `tools/go-fakecoin` directory (where this README lives):

```
> go build
```

# Running

You will need to have initialized a Filecoin repo. The default repo is in `~/.fakecoin`.

Initialize it from the `go-filecoin` directory:

```
> ./go-filecoin init --repodir=~/.fakecoin
```

This will create a new repo in`~/.fakecoin'.

Now add a fake chain.

From `tools/go-fakecoin`:

```
> ./go-fakecoin fake -length=25
```

If, for example, you want to use the standard Filecoin repo directory instead:

```
> ./go-fakecoin fake -length=25 -repodir=~/.filecoin
```


Then, from the `go-filecoin` directory, start a daemon:

```
> ./go-filecoin daemon --repodir=~/.fakecoin
```


You should be able to see the result:

```
> ./go-filecoin chain ls --enc json
```

