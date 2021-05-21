package cmd

import cmds "github.com/ipfs/go-ipfs-cmds"

var (
	AdminExtra = new(cmds.Extra).SetValue("perm", "admin")
	WriteExtra = new(cmds.Extra).SetValue("perm", "write")
	SignExtra  = new(cmds.Extra).SetValue("perm", "sign")
	ReadExtra  = new(cmds.Extra).SetValue("perm", "read")
)
