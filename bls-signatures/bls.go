package bls

import (
	"unsafe"
)

// #cgo LDFLAGS: -L${SRCDIR}/lib -lbls_signatures
// #cgo pkg-config: ${SRCDIR}/lib/pkgconfig/libbls_signatures.pc
// #include "./include/libbls_signatures.h"
import "C"

// Hash computes the digest of a message
func Hash(message Message) Digest {
	// prep request
	cMessage := C.CBytes(message)
	cMessagePtr := (*C.uchar)(cMessage)
	cMessageLen := C.size_t(len(message))
	defer C.free(cMessage)

	// call method
	resPtr := (*C.HashResponse)(unsafe.Pointer(C.hash(cMessagePtr, cMessageLen)))
	defer C.destroy_hash_response(resPtr)

	// prep response
	var digest Digest
	digestSlice := C.GoBytes(unsafe.Pointer(&resPtr.digest), DigestBytes)
	copy(digest[:], digestSlice)

	return digest
}

// Verify verifies that a signature is the aggregated signature of digests - pubkeys
func Verify(signature Signature, digests []Digest, publicKeys []PublicKey) bool {
	// prep data
	flattenedDigests := make([]byte, DigestBytes*len(digests))
	for idx, digest := range digests {
		copy(flattenedDigests[(DigestBytes*idx):(DigestBytes*(1+idx))], digest[:])
	}

	flattenedPublicKeys := make([]byte, PublicKeyBytes*len(publicKeys))
	for idx, publicKey := range publicKeys {
		copy(flattenedPublicKeys[(PublicKeyBytes*idx):(PublicKeyBytes*(1+idx))], publicKey[:])
	}

	// prep request
	cSignature := C.CBytes(signature[:])
	cSignaturePtr := (*C.uchar)(cSignature)
	defer C.free(cSignature)

	cFlattenedDigests := C.CBytes(flattenedDigests)
	cFlattenedDigestsPtr := (*C.uint8_t)(cFlattenedDigests)
	cFlattenedDigestsLen := C.size_t(len(flattenedDigests))
	defer C.free(cFlattenedDigests)

	cFlattenedPublicKeys := C.CBytes(flattenedPublicKeys)
	cFlattenedPublicKeysPtr := (*C.uint8_t)(cFlattenedPublicKeys)
	cFlattenedPublicKeysLen := C.size_t(len(flattenedPublicKeys))
	defer C.free(cFlattenedPublicKeys)

	// call method
	resPtr := (*C.VerifyResponse)(unsafe.Pointer(C.verify(cSignaturePtr, cFlattenedDigestsPtr, cFlattenedDigestsLen, cFlattenedPublicKeysPtr, cFlattenedPublicKeysLen)))
	defer C.destroy_verify_response(resPtr)

	// prep response
	var result = false

	if resPtr.result > 0 {
		result = true
	}

	return result
}

// Aggregate aggregates signatures together into a new signature
func Aggregate(signatures []Signature) Signature {
	// prep data
	flattenedSignatures := make([]byte, SignatureBytes*len(signatures))
	for idx, sig := range signatures {
		copy(flattenedSignatures[(SignatureBytes*idx):(SignatureBytes*(1+idx))], sig[:])
	}

	// prep request
	cFlattenedSignatures := C.CBytes(flattenedSignatures)
	cFlattenedSignaturesPtr := (*C.uint8_t)(cFlattenedSignatures)
	cFlattenedSignaturesLen := C.size_t(len(flattenedSignatures))
	defer C.free(cFlattenedSignatures)

	// call method
	resPtr := (*C.AggregateResponse)(unsafe.Pointer(C.aggregate(cFlattenedSignaturesPtr, cFlattenedSignaturesLen)))
	defer C.destroy_aggregate_response(resPtr)

	// prep response
	var signature Signature
	signatureSlice := C.GoBytes(unsafe.Pointer(&resPtr.signature), SignatureBytes)
	copy(signature[:], signatureSlice)

	return signature
}

// PrivateKeyGenerate generates a private key
func PrivateKeyGenerate() PrivateKey {
	// call method
	resPtr := (*C.PrivateKeyGenerateResponse)(unsafe.Pointer(C.private_key_generate()))
	defer C.destroy_private_key_generate_response(resPtr)

	// prep response
	var privateKey PrivateKey
	privateKeySlice := C.GoBytes(unsafe.Pointer(&resPtr.private_key), PrivateKeyBytes)
	copy(privateKey[:], privateKeySlice)

	return privateKey
}

// PrivateKeySign signs a message
func PrivateKeySign(privateKey PrivateKey, message Message) Signature {
	// prep request
	cPrivateKey := C.CBytes(privateKey[:])
	cPrivateKeyPtr := (*C.uchar)(cPrivateKey)
	defer C.free(cPrivateKey)

	cMessage := C.CBytes(message)
	cMessagePtr := (*C.uchar)(cMessage)
	cMessageLen := C.size_t(len(message))
	defer C.free(cMessage)

	// call method
	resPtr := (*C.PrivateKeySignResponse)(unsafe.Pointer(C.private_key_sign(cPrivateKeyPtr, cMessagePtr, cMessageLen)))
	defer C.destroy_private_key_sign_response(resPtr)

	// prep response
	var signature Signature
	signatureSlice := C.GoBytes(unsafe.Pointer(&resPtr.signature), SignatureBytes)
	copy(signature[:], signatureSlice)

	return signature
}

// PrivateKeyPublicKey gets the public key for a private key
func PrivateKeyPublicKey(privateKey PrivateKey) PublicKey {
	// prep request
	cPrivateKey := C.CBytes(privateKey[:])
	cPrivateKeyPtr := (*C.uchar)(cPrivateKey)
	defer C.free(cPrivateKey)

	// call method
	resPtr := (*C.PrivateKeyPublicKeyResponse)(unsafe.Pointer(C.private_key_public_key(cPrivateKeyPtr)))
	defer C.destroy_private_key_public_key_response(resPtr)

	// prep response
	var publicKey PublicKey
	publicKeySlice := C.GoBytes(unsafe.Pointer(&resPtr.public_key), PublicKeyBytes)
	copy(publicKey[:], publicKeySlice)

	return publicKey
}
