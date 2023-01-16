package remotewallet

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/wallet/key"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var keyMapper = map[address.Protocol]types.KeyType{
	address.SECP256K1: types.KTSecp256k1,
	address.BLS:       types.KTBLS,
	address.Delegated: types.KTDelegated,
}

func GetKeyType(p address.Protocol) types.KeyType {
	k, ok := keyMapper[p]
	if ok {
		return k
	}
	return types.KTUnknown
}

func ConvertRemoteKeyInfo(key *key.KeyInfo) *types.KeyInfo {
	return &types.KeyInfo{
		PrivateKey: key.Key(),
		Type:       types.SignType2Key(key.SigType),
	}
}

func ConvertLocalKeyInfo(keyInfo *types.KeyInfo) *key.KeyInfo {
	ki := &key.KeyInfo{
		SigType: types.KeyType2Sign(keyInfo.Type),
	}
	ki.SetPrivateKey(keyInfo.PrivateKey)

	return ki
}
