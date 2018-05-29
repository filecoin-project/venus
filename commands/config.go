package commands

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	"gx/ipfs/QmWHbPAp5UWfwZE3XCgD93xsCYZyk12tAAQVL3QXLKcWaj/toml"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

var configCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get and set filecoin config values",
		ShortDescription: `
go-filecoin config controls configuration variables.  It works similar to 
'git config'.  The configuration values are stored in a config file inside 
your filecoin repo.  Specify the key as a period separated string of object 
keys. Specify the value to set as a toml value`,
		LongDescription: `
go-filecoin config controls configuration variables.  It works similar to 
'git config'.  The configuration values are stored in a config file inside 
your filecoin repo.  Outputs are written in toml format. Specify the key as
a period separated string of object keys. Specify the value to set as a toml
value.  All subkeys including entire tables can be get and set.  Examples:

$ go-filecoin config bootstrap.addresses.0
0 = "oldaddr"

$ go-filecoin config bootstrap.addresses '["newaddr"]'
addresses = ["newaddr"]

$ go-filecoin config bootstrap.addresses.0 '"oldaddr"'
0 = "oldaddr"

$ go-filecoin config bootstrap
[bootstrap]
 addresses = ["oldaddr"]

$ go-filecoin config datastore 'type = "badgerds"
path="badger"
'
[datastore]
  type = "badgerds"
  path = "badger"

$ go-filecoin config datastore '{type="badgerds", path="badger"}'
[datastore]
  type = "badgerds"
  path = "badger"

`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, false, "The key of the config entry (e.g. \"API.Address\")."),
		cmdkit.StringArg("value", false, false, "The value to set the config entry to."),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		key := req.Arguments[0]

		var output interface{}
		defer func() {
			if output != nil {
				re.Emit(output) // nolint: errcheck
			}
		}()
		var err error
		var cf interface{}
		r := GetNode(env).Repo
		cfg := r.Config()
		if len(req.Arguments) == 2 {
			value := req.Arguments[1]
			cf, err = cfg.Set(key, value)
			if err != nil {
				return err
			}
			err = r.ReplaceConfig(cfg)
			if err != nil {
				return err
			}
			output, err = makeOutput(cf)
		} else {
			cf, err = cfg.Get(key)
			if err != nil {
				return err
			}
			output, err = makeOutput(cf)
		}
		return err
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			key := req.Arguments[0]
			vT := reflect.TypeOf(v)
			k := makeKey(key, vT)

			field := reflect.StructField{
				Name: "Field",
				Type: vT,
				Tag:  reflect.StructTag(fmt.Sprintf("toml:\"%s\"", k)),
			}
			encodeT := reflect.StructOf([]reflect.StructField{field})
			vWrap := reflect.New(encodeT)
			vWrap.Elem().Field(0).Set(reflect.ValueOf(v))
			return toml.NewEncoder(w).Encode(vWrap.Interface())
		}),
	},
}

// makeKey makes the correct display key for the TOML formatting of the given
// type. If the type serializes to a TOML table/table array the entire key
// should be returned, otherwise only the last period separated substring is
// returned
func makeKey(key string, vT reflect.Type) string {
	ks := strings.Split(key, ".")
	switch vT.Kind() {
	case reflect.Ptr, reflect.Array, reflect.Slice:
		elemT := vT.Elem()
		if elemT.Kind() == reflect.Struct || elemT.Kind() == reflect.Map {
			return key
		}
		return ks[len(ks)-1]
	case reflect.Struct, reflect.Map:
		return key
	default:
		return ks[len(ks)-1]
	}
}

// makeOutput converts struct configFields to a map[string]interface{} with
// toml tags as the string keys.  This is to preserve tag information across
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
		b := strings.Builder{}
		err := toml.NewEncoder(&b).Encode(configField)
		if err != nil {
			return nil, err
		}
		_, err = toml.Decode(b.String(), &output)
		if err != nil {
			return nil, err
		}
		return output, nil
	}

	return configField, nil
}
