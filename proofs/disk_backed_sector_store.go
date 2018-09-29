package proofs

import (
	"runtime"
	"unsafe"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

/*
#cgo LDFLAGS: -L${SRCDIR}/rust-proofs/target/release -Wl,-rpath,\$ORIGIN/lib:${SRCDIR}/rust-proofs/target/release/ -lfilecoin_proofs -lsector_base
#include "./rust-proofs/filecoin-proofs/libfilecoin_proofs.h"
#include "./rust-proofs/sector-base/libsector_base.h"

*/
import "C"

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
		dbs.destroy()
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
		dbs.destroy()
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
		dbs.destroy()
	})

	return dbs
}

// NewSealedSectorAccess dispenses a new sealed sector access
func (ss *DiskBackedSectorStore) NewSealedSectorAccess() (NewSectorAccessResponse, error) {
	// a mutable pointer to a NewSealedSectorAccessResponse C-struct
	resPtr := (*C.NewSealedSectorAccessResponse)(unsafe.Pointer(C.new_sealed_sector_access((*C.Box_SectorStore)(ss.ptr))))
	defer C.destroy_new_sealed_sector_access_response(resPtr)

	if resPtr.status_code != 0 {
		return NewSectorAccessResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	return NewSectorAccessResponse{
		SectorAccess: C.GoString(resPtr.sector_access),
	}, nil
}

// NewStagingSectorAccess dispenses a new staging sector access
func (ss *DiskBackedSectorStore) NewStagingSectorAccess() (NewSectorAccessResponse, error) {
	// a mutable pointer to a NewStagingSectorAccessResponse C-struct
	resPtr := (*C.NewStagingSectorAccessResponse)(unsafe.Pointer(C.new_staging_sector_access((*C.Box_SectorStore)(ss.ptr))))
	defer C.destroy_new_staging_sector_access_response(resPtr)

	if resPtr.status_code != 0 {
		return NewSectorAccessResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	return NewSectorAccessResponse{
		SectorAccess: C.GoString(resPtr.sector_access),
	}, nil
}

// WriteUnsealed writes bytes to an unsealed sector.
func (ss *DiskBackedSectorStore) WriteUnsealed(req WriteUnsealedRequest) (WriteUnsealedResponse, error) {
	accessCStr := C.CString(req.SectorAccess)
	defer C.free(unsafe.Pointer(accessCStr))

	cBytes := C.CBytes(req.Data)
	defer C.free(cBytes)

	// a mutable pointer to a WriteAndPreprocessResponse C-struct
	resPtr := C.write_and_preprocess((*C.Box_SectorStore)(ss.ptr), accessCStr, (*C.uint8_t)(cBytes), C.size_t(len(req.Data)))
	defer C.destroy_write_and_preprocess_response(resPtr)

	if resPtr.status_code != 0 {
		return WriteUnsealedResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	return WriteUnsealedResponse{
		NumBytesWritten: uint64(resPtr.num_bytes_written),
	}, nil
}

// TruncateUnsealed truncates the unsealed sector identified by `SectorAccess` to `NumBytes`.
func (ss *DiskBackedSectorStore) TruncateUnsealed(req TruncateUnsealedRequest) error {
	accessCStr := C.CString(req.SectorAccess)
	defer C.free(unsafe.Pointer(accessCStr))

	// a mutable pointer to a TruncateUnsealedResponse C-struct
	resPtr := C.truncate_unsealed((*C.Box_SectorStore)(ss.ptr), accessCStr, C.uint64_t(req.NumBytes))
	defer C.destroy_truncate_unsealed_response(resPtr)

	if resPtr.status_code != 0 {
		return errors.New(C.GoString(resPtr.error_msg))
	}

	return nil
}

// GetNumBytesUnsealed returns the number of bytes in an unsealed sector.
func (ss *DiskBackedSectorStore) GetNumBytesUnsealed(req GetNumBytesUnsealedRequest) (GetNumBytesUnsealedResponse, error) {
	accessCStr := C.CString(req.SectorAccess)
	defer C.free(unsafe.Pointer(accessCStr))

	// a mutable pointer to a GetNumBytesUnsealed C-struct
	resPtr := C.num_unsealed_bytes((*C.Box_SectorStore)(ss.ptr), accessCStr)
	defer C.destroy_num_unsealed_bytes_response(resPtr)

	if resPtr.status_code != 0 {
		return GetNumBytesUnsealedResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	return GetNumBytesUnsealedResponse{
		NumBytes: uint64(resPtr.num_bytes),
	}, nil
}

// GetMaxUnsealedBytesPerSector returns the number of bytes that will fit into an unsealed sector dispensed by this store.
func (ss *DiskBackedSectorStore) GetMaxUnsealedBytesPerSector() (GetMaxUnsealedBytesPerSectorResponse, error) {
	resPtr := C.max_unsealed_bytes_per_sector((*C.Box_SectorStore)(ss.ptr))
	defer C.destroy_max_unsealed_bytes_per_sector_response(resPtr)

	if resPtr.status_code != 0 {
		return GetMaxUnsealedBytesPerSectorResponse{}, errors.New(C.GoString(resPtr.error_msg))
	}

	return GetMaxUnsealedBytesPerSectorResponse{
		NumBytes: uint64(resPtr.num_bytes),
	}, nil
}

// GetCPtr returns the underlying C pointer
func (ss *DiskBackedSectorStore) GetCPtr() unsafe.Pointer {
	return ss.ptr
}

// destroy deallocates and destroys a DiskBackedSectorStore
func (ss *DiskBackedSectorStore) destroy() {
	C.destroy_storage((*C.Box_SectorStore)(ss.ptr))

	ss.ptr = nil
}
