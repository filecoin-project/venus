package wallet

import (
	"encoding/hex"
	"encoding/json"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/wallet/key"

	"github.com/pborman/uuid"
)

const (
	version = 3
)

// Key private key info
type Key struct {
	ID uuid.UUID // Version 4 "random" for unique id not derived from key data
	// to simplify lookups we also store the address
	Address address.Address
	KeyInfo *key.KeyInfo
}

type plainKey struct {
	Address string `json:"address"`
	KeyInfo string `json:"privatekey"`
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type encryptedKey struct {
	Address string     `json:"address"`
	Crypto  CryptoJSON `json:"crypto"`
	ID      string     `json:"id"`
	Version int        `json:"version"`
}

type CryptoJSON struct {
	Cipher       string                 `json:"cipher"`
	CipherText   string                 `json:"ciphertext"`
	CipherParams cipherParams           `json:"cipherparams"`
	KDF          string                 `json:"kdf"`
	KDFParams    map[string]interface{} `json:"kdfparams"`
	MAC          string                 `json:"mac"`
}

type cipherParams struct {
	IV string `json:"iv"`
}

func (k *Key) MarshalJSON() (j []byte, err error) {
	kiBytes, err := json.Marshal(k.KeyInfo)
	if err != nil {
		return nil, err
	}

	jStruct := plainKey{
		hex.EncodeToString([]byte(k.Address.String())),
		hex.EncodeToString(kiBytes),
		k.ID.String(),
		version,
	}
	j, err = json.Marshal(jStruct)
	return j, err
}

func (k *Key) UnmarshalJSON(j []byte) (err error) {
	plainKey := new(plainKey)
	err = json.Unmarshal(j, &plainKey)
	if err != nil {
		return err
	}

	u := new(uuid.UUID)
	*u = uuid.Parse(plainKey.ID)
	k.ID = *u

	addr, err := hex.DecodeString(plainKey.Address)
	if err != nil {
		return err
	}
	k.Address, err = address.NewFromString(string(addr))
	if err != nil {
		return err
	}

	k.KeyInfo = new(key.KeyInfo)
	kiBytes, err := hex.DecodeString(plainKey.KeyInfo)
	if err != nil {
		return err
	}
	err = json.Unmarshal(kiBytes, k.KeyInfo)
	if err != nil {
		return err
	}

	return nil
}
