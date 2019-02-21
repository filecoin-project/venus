// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// THIS WAS TAKEN FROM github.com/ethereum/go-ethereum MODIFY CAREFULLY

package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"reflect"
	"testing"

	"gx/ipfs/QmZp3eKdYQHHAneECmeK6HhiMwTPufmjC8DuuaGKv3unvx/blake2b-simd"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"

	cu "github.com/filecoin-project/go-filecoin/crypto/util"
)

var (
	testSignMsgHash   = blake2b.Sum256([]byte("I am a message that has been signed"))
	testUnsignMsgHash = blake2b.Sum256([]byte("I am a message that has NOT been signed"))
)

func TestEcrecover(t *testing.T) {
	assert := assert.New(t)

	key, err := GenerateKey()
	assert.NoError(err)
	pubKey, ok := key.Public().(*ecdsa.PublicKey)
	assert.True(ok)
	expKey := ECDSAPubToBytes(pubKey)

	sig, err := Sign(testSignMsgHash[:], key)
	assert.NoError(err)

	recKey, err := Ecrecover(testSignMsgHash[:], sig)
	assert.NoError(err)
	assert.Equal(expKey, recKey)

	badRecKey, err := Ecrecover(testUnsignMsgHash[:], sig)
	assert.NoError(err)
	assert.NotEqual(expKey, badRecKey)
}

func TestVerifySignature(t *testing.T) {
	assert := assert.New(t)

	// generate a private key
	genKey, err := GenerateKey()
	assert.NoError(err)

	// generate compressed and uncompressed pubkey
	pkc := cu.SerializeCompressed(&genKey.PublicKey)
	pk := cu.SerializeUncompressed(&genKey.PublicKey)

	// generate a signature with private key
	sigwRec, err := Sign(testSignMsgHash[:], genKey)
	assert.NoError(err)

	// remove recovery ID
	sig := sigwRec[:len(sigwRec)-1]

	// generate a bad signature
	badSig := append(sigwRec, 1, 2, 3)

	// generate a bad public key
	badPk := cu.SerializeCompressed(&genKey.PublicKey)
	badPk[0]++

	assert.True(VerifySignature(pk, testSignMsgHash[:], sig), "can't verify signature with uncompressed key")
	assert.True(VerifySignature(pkc, testSignMsgHash[:], sig), "can't verify signature with compressed key")

	assert.False(VerifySignature(pk, testSignMsgHash[:], badSig), "signature valid with extra bytes at the end")
	assert.False(VerifySignature(pk, testSignMsgHash[:], nil), "nil signature valid")
	assert.False(VerifySignature(pk, nil, sig), "signature valid with no message")
	assert.False(VerifySignature(nil, testSignMsgHash[:], sig), "signature valid with no key")
	assert.False(VerifySignature(pk, testSignMsgHash[:], sig[:len(sig)-2]), "signature valid even though it's incomplete")
	assert.False(VerifySignature(badPk, testSignMsgHash[:], sig), "signature valid with with wrong public key")
	assert.False(VerifySignature(pk, testUnsignMsgHash[:], sig), "signature valid with with unsigned message")
}

// This test checks that VerifySignature rejects malleable signatures with s > N/2.
func TestVerifySignatureMalleable(t *testing.T) {
	assert := assert.New(t)

	sig := cu.MustDecode("0x638a54215d80a6713c8d523a6adc4e6e73652d859103a36b700851cb0e61b66b8ebfc1a610c57d732ec6e0a8f06a9a7a28df5051ece514702ff9cdff0b11f454")
	key := cu.MustDecode("0x03ca634cae0d49acb401d8a4c6b6fe8c55b70d115bf400769cc1400f3258cd3138")
	msg := cu.MustDecode("0xd301ce462d3e639518f482c7f03821fec1e602018630ce621e1e7851c12343a6")
	assert.False(VerifySignature(key, msg, sig))
}

func TestDecompressPubkey(t *testing.T) {
	assert := assert.New(t)

	// generate a private key
	genKey, err := GenerateKey()
	assert.NoError(err)

	// generate compressed and uncompressed pubkey
	pubKey, ok := genKey.Public().(*ecdsa.PublicKey)
	assert.True(ok)
	pkc := cu.SerializeCompressed(pubKey)
	pk := cu.SerializeUncompressed(pubKey)
	badPkc := append(pkc, 1, 2, 3)

	key, err := DecompressPubkey(pkc)
	assert.NoError(err)

	uncompressed := ECDSAPubToBytes(key)
	assert.True(bytes.Equal(uncompressed, pk), "wrong public key result: got %x, want %x", uncompressed, pkc)

	_, err = DecompressPubkey(nil)
	assert.Error(err, "no error for nil pubkey")

	_, err = DecompressPubkey(pkc[:5])
	assert.Error(err, "no error for incomplete pubkey")

	_, err = DecompressPubkey(badPkc)
	assert.Error(err, "no error for pubkey with extra bytes at the end")
}

func TestCompressPubkey(t *testing.T) {
	assert := assert.New(t)

	// generate a private key
	genKey, err := GenerateKey()
	assert.NoError(err)

	// generate compressed and uncompressed pubkey
	pubKey, ok := genKey.Public().(*ecdsa.PublicKey)
	assert.True(ok)
	pkc := cu.SerializeCompressed(pubKey)

	compressed := CompressPubkey(pubKey)
	assert.True(bytes.Equal(compressed, pkc), "wrong public key result: got %x, want %x", compressed, pkc)
}

func TestPubkeyRandom(t *testing.T) {
	assert := assert.New(t)
	const runs = 200

	for i := 0; i < runs; i++ {
		key, err := GenerateKey()
		assert.NoErrorf(err, "iteration: %d", i)

		pubkey2, err := DecompressPubkey(CompressPubkey(&key.PublicKey))
		assert.NoErrorf(err, "iteration: %d", i)
		assert.Truef(reflect.DeepEqual(key.PublicKey, *pubkey2), "iteration %d: keys not equal", i)
	}
}

func BenchmarkEcrecoverSignature(b *testing.B) {
	key, err := GenerateKey()
	if err != nil {
		b.Fatal(err)
	}

	sig, err := Sign(testSignMsgHash[:], key)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Ecrecover(testSignMsgHash[:], sig); err != nil {
			b.Fatal("ecrecover error", err)
		}
	}
}

func BenchmarkVerifySignature(b *testing.B) {
	key, err := GenerateKey()
	if err != nil {
		b.Fatal(err)
	}

	// generate compressed and uncompressed pubkey
	pubKey, ok := key.Public().(*ecdsa.PublicKey)
	if !ok {
		b.FailNow()
	}

	pk := cu.SerializeUncompressed(pubKey)

	sigv, err := Sign(testSignMsgHash[:], key)
	if err != nil {
		b.Fatal(err)
	}

	sig := sigv[:len(sigv)-1] // remove recovery id
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !VerifySignature(pk, testSignMsgHash[:], sig) {
			b.Fatal("verify error")
		}
	}
}

func BenchmarkDecompressPubkey(b *testing.B) {
	key, err := GenerateKey()
	if err != nil {
		b.Fatal(err)
	}

	// generate compressed and uncompressed pubkey
	pubKey, ok := key.Public().(*ecdsa.PublicKey)
	if !ok {
		b.FailNow()
	}

	pkc := cu.SerializeCompressed(pubKey)
	for i := 0; i < b.N; i++ {
		if _, err := DecompressPubkey(pkc); err != nil {
			b.Fatal(err)
		}
	}
}
