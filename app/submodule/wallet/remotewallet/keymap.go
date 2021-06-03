package remotewallet

import (
	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/wallet"
)

var keyMapper = map[address.Protocol]wallet.KeyType{
	address.SECP256K1: wallet.KTSecp256k1,
	address.BLS:       wallet.KTBLS,
}

func GetKeyType(p address.Protocol) wallet.KeyType {
	k, ok := keyMapper[p]
	if ok {
		return k
	}
	return wallet.KTUnknown
}

func ConvertRemoteKeyInfo(key *crypto.KeyInfo) *wallet.KeyInfo {
	return &wallet.KeyInfo{
		PrivateKey: key.PrivateKey,
		Type:       wallet.SignType2Key(key.SigType),
	}
}
func ConvertLocalKeyInfo(key *wallet.KeyInfo) *crypto.KeyInfo {
	return &crypto.KeyInfo{
		PrivateKey: key.PrivateKey,
		SigType:    wallet.KeyType2Sign(key.Type),
	}
}
