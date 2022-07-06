package types

import (
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
)

func MustParseAddress(addr string) address.Address {
	ret, err := address.NewFromString(addr)
	if err != nil {
		panic(err)
	}

	return ret
}

func MustParseCid(c string) cid.Cid {
	ret, err := cid.Decode(c)
	if err != nil {
		panic(err)
	}

	return ret
}

func NewGasFeeCap(price int64) abi.TokenAmount {
	return abi.NewTokenAmount(price)
}

func NewGasPremium(price int64) abi.TokenAmount {
	return abi.NewTokenAmount(price)
}

// BlockTopic returns the network pubsub topic identifier on which new blocks are announced.
func BlockTopic(networkName string) string {
	return fmt.Sprintf("/fil/blocks/%s", networkName)
}

// MessageTopic returns the network pubsub topic identifier on which new messages are announced.
// The message payload is just a SignedMessage.
func MessageTopic(networkName string) string {
	return fmt.Sprintf("/fil/msgs/%s", networkName)
}

func IndexerIngestTopic(networkName string) string {
	// The network name testnetnet is here for historical reasons.
	// Going forward we aim to use the name `mainnet` where possible.
	if networkName == "testnetnet" {
		networkName = "mainnet"
	}

	return "/indexer/ingest/" + networkName
}

func DrandTopic(chainInfoJSON string) (string, error) {
	var drandInfo = struct {
		Hash string `json:"hash"`
	}{}
	err := json.Unmarshal([]byte(chainInfoJSON), &drandInfo)
	if err != nil {
		return "", fmt.Errorf("could not unmarshal drand chain info: %w", err)
	}
	return "/drand/pubsub/v0.0.0/" + drandInfo.Hash, nil
}
