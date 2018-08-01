package commands

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	"gx/ipfs/QmWHbPAp5UWfwZE3XCgD93xsCYZyk12tAAQVL3QXLKcWaj/toml"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {

		var output interface{}
		defer func() {
			if output != nil {
				re.Emit(output) // nolint: errcheck
			}
		}()

		r := GetNode(env).Repo
		cfg := r.Config()

		key := req.Arguments[0]
		if len(req.Arguments) == 2 {
			value := req.Arguments[1]
			cf, err := cfg.Set(key, value)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			if err := r.ReplaceConfig(cfg); err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			output, err := makeOutput(cf)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}
			re.Emit(output) // nolint: errcheck
		} else {
			cf, err := cfg.Get(key)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			output, err := makeOutput(cf)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}
			re.Emit(output) // nolint: errcheck
		}
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
