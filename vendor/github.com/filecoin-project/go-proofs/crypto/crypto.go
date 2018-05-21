package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"math/big"

	"github.com/filecoin-project/go-proofs/data"
	"github.com/onrik/gomerkle"
)

// MerkleProof TODO
type MerkleProof struct {
	Data []byte
	Hash []byte
	Path gomerkle.Proof
}

// Crh TODO
func Crh(x []byte) []byte {
	h := sha256.New()
	h.Write(x) // nolint: errcheck
	bs := h.Sum(nil)
	return bs
}

// Kdf is a key-derivation function.
func Kdf(ciphertexts []byte) []byte {
	h := sha256.New()
	h.Write(ciphertexts) // nolint: errcheck
	bs := h.Sum(nil)
	return bs
}

// Xor computes and returns the exclusive or of input and key.
func Xor(input, key []byte) (output []byte) {
	for i := 0; i < len(input); i++ {
		output = append(output, input[i]^key[i%len(key)])
	}

	return output
	// return input
}

// AESEnc performs AES encryption of plaintext, using key.
func AESEnc(key []byte, plaintext []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	ciphertext := make([]byte, len(plaintext))
	iv := make([]byte, aes.BlockSize)

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintext)
	return ciphertext
}

// AESDec performs AES decryption of ciphertext, using key.
func AESDec(key []byte, ciphertext []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	plaintext := make([]byte, len(ciphertext))
	iv := make([]byte, aes.BlockSize)
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	return plaintext
}

// XorEncDec TODO
func XorEncDec(id []byte, key []byte, plaintext []byte) []byte {
	hash := Crh(append(key, id...))
	return Xor(plaintext, hash)
}

// SlothIterativeEnc TODO
func SlothIterativeEnc(key []byte, plaintext []byte, p *big.Int, v *big.Int, t int) []byte {
	prev := plaintext
	for i := 0; i < t; i++ {
		tmp := SlothEnc(key, prev, p, v)
		// fmt.Println("ENC iter", i, "\nfrom\t", prev, len(prev), "\nto\t", tmp, len(tmp))
		prev = tmp
	}
	return prev
}

// SlothIterativeDec TODO
func SlothIterativeDec(key []byte, ciphertext []byte, p *big.Int, t int) []byte {
	prev := ciphertext
	for i := 0; i < t; i++ {
		tmp := SlothDec(key, prev, p)
		// fmt.Println("DEC iter", i, "\nfrom\t", prev, len(prev), "\nto\t", tmp, len(tmp))
		prev = tmp
	}
	return prev
}

// MiMCEnc TODO
func MiMCEnc(key []byte, plaintext []byte, p *big.Int, v *big.Int) []byte {
	// Convert plaintext into a number
	x := (&big.Int{}).SetBytes(plaintext)
	// Convert key into a number
	k := (&big.Int{}).SetBytes(key)

	// Compute (x+k)^v mod p
	res := &big.Int{}
	res.Add(x, k).Mod(res, p).Exp(res, v, p)

	return res.Bytes()
}

// MiMCDec TODO
func MiMCDec(key []byte, ciphertext []byte, p *big.Int) []byte {
	// Convert ciphertext into a number
	c := (&big.Int{}).SetBytes(ciphertext)
	// Convert key into a number
	k := (&big.Int{}).SetBytes(key)

	// Compute c^3 - k mod p
	res := &big.Int{}
	three := big.NewInt(int64(3))
	res.Exp(c, three, p).Sub(res, k).Mod(res, p)

	return res.Bytes()
}

// SlothEnc TODO
func SlothEnc(key []byte, plaintext []byte, p *big.Int, v *big.Int) []byte {
	// Compute E(k, x) = AES.Encrypt(k, (x+k)^v mod p)
	enc := MiMCEnc(key, plaintext, p, v)
	return AESEnc(key, enc)
}

// SlothDec TODO
func SlothDec(key []byte, ciphertext []byte, p *big.Int) []byte {
	// Compute D(k, c) = AES.Decrypt(k, c)^3 - k mod p
	dec := AESDec(key, ciphertext)
	return MiMCDec(key, dec, p)
}

// Enc TODO
func Enc(id []byte, key []byte, plaintext []byte) []byte {
	return AESEnc(id, Xor(plaintext, key))
}

// Dec TODO
func Dec(id []byte, key []byte, ciphertext []byte) []byte {
	plaintext := AESDec(id, ciphertext)
	return Xor(plaintext, key)
}

// DataAtNodeOffset calculates and returns the offset of data at position v, given element size, nodeSize.
func DataAtNodeOffset(v, nodeSize int) int {
	return (v - 1) * nodeSize
}

// DataAtNode returns the byte slice representing one node (of uniform size, nodeSize) at position v of data.
func DataAtNode(data data.ReadWriter, v, nodeSize int) ([]byte, error) {
	offset := DataAtNodeOffset(v, nodeSize)

	out := make([]byte, nodeSize)
	err := data.DataAt(uint64(offset), uint64(nodeSize), func(b []byte) error {
		copy(out, b)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}
