// Copyright 2014 The go-ethereum Authors
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
	"crypto/ecdsa"
	"testing"

	"gx/ipfs/QmZp3eKdYQHHAneECmeK6HhiMwTPufmjC8DuuaGKv3unvx/blake2b-simd"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestToECDSAErrors(t *testing.T) {
	assert := assert.New(t)

	_, err := HexToECDSA("0000000000000000000000000000000000000000000000000000000000000000")
	assert.Error(err)

	_, err = HexToECDSA("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	assert.Error(err)
}

func TestSign(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Generate a key pair and convert the public key to bytes for easy comparison
	key, err := GenerateKey()
	assert.NoError(err)
	ecdasKey, ok := key.Public().(*ecdsa.PublicKey)
	require.True(ok)
	// This is the key we will compare recovered keys against
	expPub := ECDSAPubToBytes(ecdasKey)

	// Sign a message using the key we generated previously
	msg := blake2b.Sum256([]byte("have you considered not asking passive aggressive questions?"))
	sig, err := Sign(msg[:], key)
	assert.NoError(err)

	// recover a public key from a message and a signature, it should be
	// equal to the previously generated public key
	recPub, err := Ecrecover(msg[:], sig)
	assert.NoError(err)
	assert.Equal(expPub, recPub)

	// should be equal to SigToPub since Ecrecover calls SigToPub under the hood
	recoveredPub2, err := SigToPub(msg[:], sig)
	assert.NoError(err)
	recPub2 := ECDSAPubToBytes(recoveredPub2)
	assert.Equal(expPub, recPub2)

	// Basic sanity check, don't sign the message, pub keys should be different
	evilMsg := blake2b.Sum256([]byte("being the detective in a crime movie where you are also the murderer..debugging"))
	recPub, err = Ecrecover(evilMsg[:], sig)
	assert.NoError(err)
	assert.NotEqual(expPub, recPub)

	recoveredPub2, err = SigToPub(evilMsg[:], sig)
	assert.NoError(err)
	recPub2 = ECDSAPubToBytes(recoveredPub2)
	assert.NotEqual(expPub, recPub2)
}

func TestInvalidSign(t *testing.T) {
	assert := assert.New(t)
	_, err := Sign(make([]byte, 1), nil)
	assert.Error(err)
	_, err = Sign(make([]byte, 33), nil)
	assert.Error(err)
}
