package pluginmockfilecoin

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/ipfs/iptb/testbed/interfaces"
	"github.com/ipfs/iptb/util"
)

// PluginName is the name of the plugin
var PluginName = "mockfilecoin"

// Mockfilecoin is a mock structure used for testing things that use go-filecoin iptb plugins
type Mockfilecoin struct {
	dir string

	stderr bytes.Buffer
	stdout bytes.Buffer
}

var NewNode testbedi.NewNodeFunc // nolint: golint

func init() {
	NewNode = func(dir string, attrs map[string]string) (testbedi.Core, error) {
		return &Mockfilecoin{
			dir: dir,
		}, nil
	}
}

// Init is not implemented
func (m *Mockfilecoin) Init(ctx context.Context, args ...string) (testbedi.Output, error) {
	return nil, nil
}

// Start is not implemented
func (m *Mockfilecoin) Start(ctx context.Context, wait bool, args ...string) (testbedi.Output, error) {
	return nil, nil
}

// Stop is not implemented
func (m *Mockfilecoin) Stop(ctx context.Context) error {
	return nil
}

// RunCmd will return "string" for args "", json for args "json", and ldjson for args "ldjson"
func (m *Mockfilecoin) RunCmd(ctx context.Context, stdin io.Reader, args ...string) (testbedi.Output, error) {
	if args[0] == "" {
		return iptbutil.NewOutput(args, []byte("string"), []byte{}, 0, nil), nil
	} else if args[0] == "json" {
		//return json object
		return iptbutil.NewOutput(args, []byte(`{"key":"value"}`), []byte{}, 0, nil), nil
	} else if args[0] == "ldjson" {
		// return ldjson objects
		return iptbutil.NewOutput(args, []byte("{\"key\":\"value1\"}\n{\"key\":\"value2\"}\n"), []byte{}, 0, nil), nil
	} else if args[0] == "add-to-daemonstderr" {
		for _, arg := range args[1:] {
			m.stderr.WriteString(arg)
			m.stderr.WriteByte('\n')
		}

		return iptbutil.NewOutput(args, []byte{}, []byte{}, 0, nil), nil
	}
	return nil, errors.New(`invalid mock args, can only be one of: "", "json", or "ldjson"`)
}

// Connect is not implemented
func (m *Mockfilecoin) Connect(ctx context.Context, n testbedi.Core) error {
	return nil
}

// Shell is not implemented
func (m *Mockfilecoin) Shell(ctx context.Context, ns []testbedi.Core) error {
	panic("not implemented")
}

// Dir is not implemented
func (m *Mockfilecoin) Dir() string {
	return m.dir
}

// Type is not implemented
func (m *Mockfilecoin) Type() string {
	panic("not implemented")
}

// String is not implemented
func (m *Mockfilecoin) String() string {
	return "mockNode"
}

// PeerID is not implemented
func (m *Mockfilecoin) PeerID() (string, error) {
	panic("not implemented")
}

// APIAddr is not implemented
func (m *Mockfilecoin) APIAddr() (string, error) {
	panic("not implemented")
}

// SwarmAddrs is not implemented
func (m *Mockfilecoin) SwarmAddrs() ([]string, error) {
	panic("not implemented")
}

// Config is not implemented
func (m *Mockfilecoin) Config() (interface{}, error) {
	panic("not implemented")
}

// WriteConfig is not implemented
func (m *Mockfilecoin) WriteConfig(interface{}) error {
	panic("not implemented")
}
