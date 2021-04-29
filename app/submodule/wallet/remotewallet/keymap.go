package remotewallet

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus-wallet/core"
	"github.com/filecoin-project/venus/pkg/crypto"
)

var keyMapper = map[address.Protocol]core.KeyType{
	address.SECP256K1: core.KTSecp256k1,
	address.BLS:       core.KTBLS,
}

func GetKeyType(p address.Protocol) core.KeyType {
	k, ok := keyMapper[p]
	if ok {
		return k
	}
	return core.KTUnknown
}

func ConvertRemoteKeyInfo(key *crypto.KeyInfo) *core.KeyInfo {
	// TODO: A better way to handle private keys is needed
	return &core.KeyInfo{
		PrivateKey: key.Key(),
		Type:       core.SignType2Key(key.SigType),
	}
}
func ConvertLocalKeyInfo(key *core.KeyInfo) *crypto.KeyInfo {
	ki := &crypto.KeyInfo{
		SigType: core.KeyType2Sign(key.Type),
	}
	ki.SetPrivateKey(key.PrivateKey)

	return ki
}
