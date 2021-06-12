package crypto_test

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/venus/pkg/crypto"
)

func TestKeyInfoAddress(t *testing.T) {
	prv, _ := hex.DecodeString("2a2a2a2a2a2a2a2a5fbf0ed0f8364c01ff27540ecd6669ff4cc548cbe60ef5ab")
	ki := &crypto.KeyInfo{
		SigType: crypto.SigTypeSecp256k1,
	}
	ki.SetPrivateKey(prv)

	sign, _ := crypto.Sign([]byte("hello filecoin"), prv, crypto.SigTypeSecp256k1)
	t.Logf("%x", sign)
}

func TestKeyInfoUnmarshalAndMarshal(t *testing.T) {
	prv := []byte("marshal_and_unmarshal")
	prvCp := make([]byte, len(prv))
	copy(prvCp, prv)
	ki := &crypto.KeyInfo{
		SigType: crypto.SigTypeSecp256k1,
	}
	ki.SetPrivateKey(prv)

	assert.NotNil(t, ki.PrivateKey)
	t.Log(string(prv))
	assert.Equal(t, prvCp, ki.Key())

	kiByte, err := json.Marshal(ki)
	assert.NoError(t, err)

	var newKI crypto.KeyInfo
	assert.NoError(t, json.Unmarshal(kiByte, &newKI))

	assert.Equal(t, ki.Key(), newKI.Key())
	assert.Equal(t, ki.SigType, newKI.SigType)
}
