package commands

import (
	"encoding/json"
	"reflect"

	"gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
)

var configCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get and set filecoin config values",
		ShortDescription: `
go-filecoin config controls configuration variables. It works similar to
'git config'. The configuration values are stored in a config file inside
your filecoin repo. When getting values, a key should be provided, like so:

go-filecoin config KEY

When setting values, the key should be given first, followed by the value and
separated by a space, like so:

go-filecoin config KEY VALUE

Specify the key as a period separated string of object keys. Specify the value
to set as a JSON value.`,
		LongDescription: `
go-filecoin config controls configuration variables. It works similar to
'git config'. The configuration values are stored in a config file inside
your filecoin repo. Outputs are written in JSON format. Specify the key as
a period separated string of object keys. Specify the value to set as a JSON
value. All subkeys including entire tables can be get and set. Examples:

$ go-filecoin config bootstrap.addresses '["newaddr"]'
["newaddr"]

$ go-filecoin config bootstrap
{"addresses":["newaddr"]}

$ go-filecoin config datastore '{"type":"badgerds","path":"badger"}'
{"path":"badger","type":"badgerds"}
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, false, "The key of the config entry (e.g. \"API.Address\")."),
		cmdkit.StringArg("value", false, false, "The value to set the config entry to."),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		api := GetAPI(env).Config()
		key := req.Arguments[0]

		if len(req.Arguments) == 2 {
			value := req.Arguments[1]
			res, err := api.Set(key, value)

			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			output, err := makeOutput(res)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			re.Emit(output) // nolint: errcheck
		} else {
			res, err := api.Get(key)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			output, err := makeOutput(res)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			re.Emit(output) // nolint: errcheck
		}
	},
}

// makeOutput converts struct configFields to a map[string]interface{} with
// JSON tags as the string keys.  This is to preserve tag information across
// daemon calls in the presence of many different possible configField types
func makeOutput(configField interface{}) (interface{}, error) {
	cfV := reflect.ValueOf(configField)
	cfT := cfV.Type()
	if cfT.Kind() == reflect.Ptr {
		cfV = cfV.Elem()
		cfT = cfT.Elem()
	}

	if cfT.Kind() == reflect.Struct {
		var output interface{}
		jsonOut, err := json.Marshal(configField)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(jsonOut, &output)
		if err != nil {
			return nil, err
		}
		return output, nil
	}

	return configField, nil
}
