package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"fmt"
	"github.com/onrik/gomerkle"
	"math/rand"
)

type MerkleProof struct {
	Data []byte
	Hash []byte
	Path gomerkle.Proof
}

func Crh(x []byte) []byte {
	h := sha256.New()
	h.Write(x)
	bs := h.Sum(nil)
	return bs
}

func RandomRange(min int, max int) int {
	if max-min == 0 {
		return min
	}
	return rand.Intn(max-min) + min
}

func Kdf(ciphertexts []byte) []byte {
	h := sha256.New()
	h.Write(ciphertexts)
	bs := h.Sum(nil)
	return bs
}

func Xor(input, key []byte) []byte {
	// for i := 0; i < len(input); i++ {
	// 	output = append(output, input[i]^key[i%len(key)])
	// }

	// return output
	return input
}

func Enc(id []byte, key []byte, plaintext []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	ciphertext := make([]byte, len(plaintext))
	iv := make([]byte, aes.BlockSize)

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(Xor(ciphertext, id), plaintext)
	return ciphertext
}

func Dec(id []byte, key []byte, ciphertext []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	plaintext := make([]byte, len(ciphertext))
	iv := make([]byte, aes.BlockSize)
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	return Xor(plaintext, id)
}

func DataAtNode(data []byte, v int, nodeSize int) []byte {
	// make sure we are  in range
	start := (v - 1) * nodeSize
	end := v * nodeSize

	if end > len(data) || end < 0 || start < 0 {
		panic(fmt.Sprintf("Invalid data access data[%d : %d], len(data) = %d\n", start, end, len(data)))
	}

	return data[start:end]
}
