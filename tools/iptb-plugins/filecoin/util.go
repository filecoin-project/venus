package filecoin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	logging "github.com/ipfs/go-log"
	"github.com/multiformats/go-multiaddr"

	"github.com/ipfs/iptb/testbed/interfaces"
)

var log = logging.Logger("util")

// WaitOnAPI waits for a nodes api to come up.
func WaitOnAPI(l testbedi.Libp2p) error {
	for i := 0; i < 50; i++ {
		err := tryAPICheck(l)
		if err == nil {
			return nil
		}
		log.Warn(err.Error())
		time.Sleep(time.Millisecond * 400)
	}

	pcid, err := l.PeerID()
	if err != nil {
		return err
	}

	return fmt.Errorf("node %s failed to come online in given time period", pcid)
}

func tryAPICheck(l testbedi.Libp2p) error {
	addrStr, err := l.APIAddr()
	if err != nil {
		return err
	}

	addr, err := multiaddr.NewMultiaddr(addrStr)
	if err != nil {
		return err
	}

	//TODO(tperson) ipv6
	ip, err := addr.ValueForProtocol(multiaddr.P_IP4)
	if err != nil {
		return err
	}
	pt, err := addr.ValueForProtocol(multiaddr.P_TCP)
	if err != nil {
		return err
	}

	resp, err := http.Get(fmt.Sprintf("http://%s:%s/api/id", ip, pt))
	if err != nil {
		return err
	}

	out := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return fmt.Errorf("liveness check failed: %s", err)
	}

	id, ok := out["ID"]
	if !ok {
		return fmt.Errorf("liveness check failed: ID field not present in output")
	}

	pcid, err := l.PeerID()
	if err != nil {
		return err
	}

	idstr, ok := id.(string)
	if !ok {
		return fmt.Errorf("liveness check failed: ID field is unexpected type")
	}

	if idstr != pcid {
		return fmt.Errorf("liveness check failed: unexpected peer at endpoint")
	}

	return nil
}

// GetAPIAddrFromRepo reads the api address from the `api` file in a nodes repo.
func GetAPIAddrFromRepo(dir string) (multiaddr.Multiaddr, error) {
	addrStr, err := ioutil.ReadFile(filepath.Join(dir, "api"))
	if err != nil {
		return nil, err
	}

	maddr, err := multiaddr.NewMultiaddr(string(addrStr))
	if err != nil {
		return nil, err
	}

	return maddr, nil
}

// UpdateOrAppendEnv will look through an array of strings for the environment key
// updating if it is found, or appending to the end if not.
func UpdateOrAppendEnv(envs []string, key, value string) []string {
	entry := fmt.Sprintf("%s=%s", key, value)

	for i, e := range envs {
		if strings.HasPrefix(e, key+"=") {
			envs[i] = entry
			return envs
		}
	}

	return append(envs, entry)
}
