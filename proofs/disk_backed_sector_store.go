package proofs

import (
	"runtime"
	"unsafe"

	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

/*
#cgo LDFLAGS: -L${SRCDIR}/rust-proofs/target/release -Wl,-rpath,${SRCDIR}/rust-proofs/target/release/ -lfilecoin_proofs -lsector_base
#include "./rust-proofs/filecoin-proofs/libfilecoin_proofs.h"
#include "./rust-proofs/sector-base/libsector_base.h"

*/
import "C"

var log = logging.Logger("DiskBackedSectorStore")

// DiskBackedSectorStore is a struct which serves as a proxy for a DiskBackedSectorStore in Rust
type DiskBackedSectorStore struct {
	ptr unsafe.Pointer
}

var _ SectorStore = &DiskBackedSectorStore{}

// NewDiskBackedSectorStore allocates and returns a new SectorStore instance. This is store that should be used by
// someone operating a real Filecoin node. The returned object should be passed to FPS operations. Note that when using
// this constructor that FPS operations can take a very long time to complete.
//
// Note: Staging and sealed directories can be the same.
func NewDiskBackedSectorStore(staging string, sealed string) *DiskBackedSectorStore {
	stagingDirPath := C.CString(staging)
	defer C.free(unsafe.Pointer(stagingDirPath))

	sealedDirPath := C.CString(sealed)
	defer C.free(unsafe.Pointer(sealedDirPath))

	ptr := C.init_new_sector_store(stagingDirPath, sealedDirPath)

	dbs := &DiskBackedSectorStore{
		ptr: unsafe.Pointer(ptr),
	}

	// Note: When the GC finds an unreachable block with an associated finalizer,
	// it clears the association and runs finalizer(obj) in a separate goroutine.
	runtime.SetFinalizer(dbs, func(dbs *DiskBackedSectorStore) {
		if err := dbs.destroy(); err != nil {
			log.Error(err)
		}
	})

	return dbs
}

// NewTestSectorStore allocates and returns a new SectorStore instance which is suitable for end-to-end testing. This
// store does not fully exercise the proofs code.
//
// Note: Staging and sealed directories can be the same.
func NewTestSectorStore(staging string, sealed string) *DiskBackedSectorStore {
	stagingDirPath := C.CString(staging)
	defer C.free(unsafe.Pointer(stagingDirPath))

	sealedDirPath := C.CString(sealed)
	defer C.free(unsafe.Pointer(sealedDirPath))

	ptr := C.init_new_test_sector_store(stagingDirPath, sealedDirPath)

	dbs := &DiskBackedSectorStore{
		ptr: unsafe.Pointer(ptr),
	}

	runtime.SetFinalizer(dbs, func(dbs *DiskBackedSectorStore) {
		if err := dbs.destroy(); err != nil {
			log.Error(err)
		}
	})

	return dbs
}

// NewProofTestSectorStore allocates and returns a new SectorStore instance which is suitable for end-to-end testing.
// This store is configured with a small sector size and can be used in automated tests which require exercising the
// proofs code.
//
// Note: Staging and sealed directories can be the same.
func NewProofTestSectorStore(staging string, sealed string) *DiskBackedSectorStore {
	stagingDirPath := C.CString(staging)
	defer C.free(unsafe.Pointer(stagingDirPath))

	sealedDirPath := C.CString(sealed)
	defer C.free(unsafe.Pointer(sealedDirPath))

	ptr := C.init_new_proof_test_sector_store(stagingDirPath, sealedDirPath)

	dbs := &DiskBackedSectorStore{
		ptr: unsafe.Pointer(ptr),
	}

	runtime.SetFinalizer(dbs, func(dbs *DiskBackedSectorStore) {
		if err := dbs.destroy(); err != nil {
			log.Error(err)
		}
	})

	return dbs
}

// NewSealedSectorAccess dispenses a new sealed sector access
func (ss *DiskBackedSectorStore) NewSealedSectorAccess() (NewSectorAccessResponse, error) {
	if ss.ptr == nil {
		return NewSectorAccessResponse{}, errors.New("NewSealedSectorAccess() - ss.ptr is nil")
	}

	var result *C.char
	defer C.free(unsafe.Pointer(result))

	code := C.new_sealed_sector_access((*C.Box_SectorStore)(ss.ptr), &result)
	if code != 0 {
		return NewSectorAccessResponse{}, errors.New(errorString(code))
	}

	return NewSectorAccessResponse{
		SectorAccess: C.GoString(result),
	}, nil
}

// NewStagingSectorAccess dispenses a new staging sector access
func (ss *DiskBackedSectorStore) NewStagingSectorAccess() (NewSectorAccessResponse, error) {
	if ss.ptr == nil {
		return NewSectorAccessResponse{}, errors.New("NewStagingSectorAccess() - ss.ptr is nil")
	}

	var result *C.char
	defer C.free(unsafe.Pointer(result))

	code := C.new_staging_sector_access((*C.Box_SectorStore)(ss.ptr), &result)
	if code != 0 {
		return NewSectorAccessResponse{}, errors.New(errorString(code))
	}

	return NewSectorAccessResponse{
		SectorAccess: C.GoString(result),
	}, nil
}

// WriteUnsealed writes bytes to an unsealed sector.
func (ss *DiskBackedSectorStore) WriteUnsealed(req WriteUnsealedRequest) (WriteUnsealedResponse, error) {
	if ss.ptr == nil {
		return WriteUnsealedResponse{}, errors.New("WriteUnsealedRequest() - ss.ptr is nil")
	}

	accessCStr := C.CString(req.SectorAccess)
	defer C.free(unsafe.Pointer(accessCStr))

	// The write_unsealed function will write to bytesWrittenPtr to indicate
	// the number of bytes which have been written to the unsealed sector.
	var bytesWritten uint64
	bytesWrittenPtr := (*C.uint64_t)(unsafe.Pointer(&bytesWritten))

	// Copy the Go byte slice into C memory
	data := C.CBytes(req.Data)
	defer C.free(data)

	code := C.write_unsealed((*C.Box_SectorStore)(ss.ptr), accessCStr, (*C.uint8_t)(data), C.size_t(len(req.Data)), bytesWrittenPtr)
	if code != 0 {
		return WriteUnsealedResponse{}, errors.New(errorString(code))
	}

	return WriteUnsealedResponse{
		NumBytesWritten: bytesWritten,
	}, nil
}

// TruncateUnsealed truncates the unsealed sector identified by `SectorAccess` to `NumBytes`.
func (ss *DiskBackedSectorStore) TruncateUnsealed(req TruncateUnsealedRequest) error {
	if ss.ptr == nil {
		return errors.New("TruncateUnsealed() - ss.ptr is nil")
	}

	accessCStr := C.CString(req.SectorAccess)
	defer C.free(unsafe.Pointer(accessCStr))

	code := C.truncate_unsealed((*C.Box_SectorStore)(ss.ptr), accessCStr, C.uint64_t(req.NumBytes))
	if code != 0 {
		return errors.New(errorString(code))
	}

	return nil
}

// GetNumBytesUnsealed returns the number of bytes in an unsealed sector.
func (ss *DiskBackedSectorStore) GetNumBytesUnsealed(req GetNumBytesUnsealedRequest) (GetNumBytesUnsealedResponse, error) {
	if ss.ptr == nil {
		return GetNumBytesUnsealedResponse{}, errors.New("GetNumBytesUnsealed() - ss.ptr is nil")
	}

	accessCStr := C.CString(req.SectorAccess)
	defer C.free(unsafe.Pointer(accessCStr))

	// num_unsealed_bytes will write to result pointer indicating the number of
	// bytes in the unsealed sector
	var numBytes uint64
	numBytesPtr := (*C.uint64_t)(unsafe.Pointer(&numBytes))

	code := C.num_unsealed_bytes((*C.Box_SectorStore)(ss.ptr), accessCStr, numBytesPtr)
	if code != 0 {
		return GetNumBytesUnsealedResponse{}, errors.New(errorString(code))
	}

	return GetNumBytesUnsealedResponse{
		NumBytes: numBytes,
	}, nil
}

// GetMaxUnsealedBytesPerSector returns the number of bytes that will fit into an unsealed sector dispensed by this store.
func (ss *DiskBackedSectorStore) GetMaxUnsealedBytesPerSector() (GetMaxUnsealedBytesPerSectorResponse, error) {
	if ss.ptr == nil {
		return GetMaxUnsealedBytesPerSectorResponse{}, errors.New("GetMaxUnsealedBytesPerSectorResponse() - ss.ptr is nil")
	}

	// max_unsealed_bytes_per_sector will write to this
	var numBytes uint64
	numBytesPtr := (*C.uint64_t)(unsafe.Pointer(&numBytes))

	code := C.max_unsealed_bytes_per_sector((*C.Box_SectorStore)(ss.ptr), numBytesPtr)
	if code != 0 {
		return GetMaxUnsealedBytesPerSectorResponse{}, errors.New(errorString(code))
	}

	return GetMaxUnsealedBytesPerSectorResponse{
		NumBytes: numBytes,
	}, nil
}

// GetCPtr returns the underlying C pointer
func (ss *DiskBackedSectorStore) GetCPtr() unsafe.Pointer {
	return ss.ptr
}

// destroy deallocates and destroys a DiskBackedSectorStore
func (ss *DiskBackedSectorStore) destroy() error {
	if ss.ptr == nil {
		return errors.New("destroy() - already destroyed")
	}

	// TODO: we need to be absolutely sure that this will not leak memory,
	// specifically the string-fields allocated by Rust (see issue #781)
	code := C.destroy_storage((*C.Box_SectorStore)(ss.ptr))
	if code != 0 {
		return errors.New(errorString(code))
	}

	ss.ptr = nil

	return nil
}
