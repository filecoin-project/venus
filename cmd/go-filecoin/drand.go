package commands

import (
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var drandCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Explore access and configure drand.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"configure": drandConfigure,
		// "random":    drandFetch,
	},
}

var drandConfigure = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Configure drand client",
		ShortDescription: `Fetches drand group configuration from one or more server. When found, it updates 
			drand client to use configuration and persists configuration in node config`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("addresses", true, true, "Addresses used to contact drand group for configuration."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("override-addrs", "use the provided addresses rather than the retrieved config to contact drand"),
		cmdkit.BoolOption("insecure", "use insecure protocol to contact drand"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		insecure, _ := req.Options["insecure"].(bool)
		override, _ := req.Options["override-addrs"].(bool)

		err := GetDrandAPI(env).Configure(req.Arguments, !insecure, override)
		if err != nil {
			return err
		}
		return re.Emit("drand group key configured")
	},
}
