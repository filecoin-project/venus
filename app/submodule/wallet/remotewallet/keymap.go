package remotewallet

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var keyMapper = map[address.Protocol]types.KeyType{
	address.SECP256K1: types.KTSecp256k1,
	address.BLS:       types.KTBLS,
}

func GetKeyType(p address.Protocol) types.KeyType {
	k, ok := keyMapper[p]
	if ok {
		return k
	}
	return types.KTUnknown
}

func ConvertRemoteKeyInfo(key *crypto.KeyInfo) *types.KeyInfo {
	return &types.KeyInfo{
		PrivateKey: key.Key(),
		Type:       types.SignType2Key(key.SigType),
	}
}
func ConvertLocalKeyInfo(key *types.KeyInfo) *crypto.KeyInfo {
	ki := &crypto.KeyInfo{
		SigType: types.KeyType2Sign(key.Type),
	}
	ki.SetPrivateKey(key.PrivateKey)

	return ki
}
