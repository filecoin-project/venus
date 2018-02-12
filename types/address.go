package types

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// TODO: make address a little more sophisticated
type Address string

func (a Address) String() string {
	return "0x" + hex.EncodeToString([]byte(a))
}

func ParseAddress(s string) (Address, error) {
	if !strings.HasPrefix(s, "0x") {
		return "", fmt.Errorf("addresses must start with 0x")
	}
	raw, err := hex.DecodeString(s[2:])
	if err != nil {
		return "", errors.Wrap(err, "decoding address failed")
	}

	return Address(raw), nil
}
