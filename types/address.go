package types

import (
	"encoding/hex"
	"fmt"
	"strings"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

// Address is the go type that represents an address in the filecoin network.
// TODO: make address a little more sophisticated
type Address string

func (a Address) String() string {
	return "0x" + hex.EncodeToString([]byte(a))
}

// ParseAddress tries to parse a given string into a filecoin address.
func ParseAddress(s string) (Address, error) {
	if !strings.HasPrefix(s, "0x") {
		return "", fmt.Errorf("addresses must start with 0x, got %s", s)
	}
	raw, err := hex.DecodeString(s[2:])
	if err != nil {
		return "", errors.Wrapf(err, "decoding address failed: %s", s)
	}

	return Address(raw), nil
}
