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
	ptr *C.Box_SectorStore
}

var _ SectorStore = &DiskBackedSectorStore{}

// NewDiskBackedSectorStore allocates and returns a new DiskBackedSectorStore
func NewDiskBackedSectorStore(staging string, sealed string) *DiskBackedSectorStore {
	stagingDirPath := C.CString(staging)
	defer C.free(unsafe.Pointer(stagingDirPath))

	sealedDirPath := C.CString(sealed)
	defer C.free(unsafe.Pointer(sealedDirPath))

	ptr := C.init_disk_backed_storage(stagingDirPath, sealedDirPath)

	dbs := &DiskBackedSectorStore{
		ptr: ptr,
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

// NewSealedSectorAccess dispenses a new sealed sector access
func (ss *DiskBackedSectorStore) NewSealedSectorAccess() (NewSectorAccessResponse, error) {
	if ss.ptr == nil {
		return NewSectorAccessResponse{}, errors.New("NewSealedSectorAccess() - ss.ptr is nil")
	}

	p := C.new_sealed_sector_access(ss.ptr)
	defer C.free(unsafe.Pointer(p))

	return NewSectorAccessResponse{
		SectorAccess: C.GoString(p),
	}, nil
}

// NewStagingSectorAccess dispenses a new staging sector access
func (ss *DiskBackedSectorStore) NewStagingSectorAccess() (NewSectorAccessResponse, error) {
	if ss.ptr == nil {
		return NewSectorAccessResponse{}, errors.New("NewStagingSectorAccess() - ss.ptr is nil")
	}

	p := C.new_staging_sector_access(ss.ptr)
	defer C.free(unsafe.Pointer(p))

	return NewSectorAccessResponse{
		SectorAccess: C.GoString(p),
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

	code := C.write_unsealed(ss.ptr, accessCStr, (*C.uint8_t)(data), C.size_t(len(req.Data)), bytesWrittenPtr)
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

	code := C.truncate_unsealed(ss.ptr, accessCStr, C.uint64_t(req.NumBytes))
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

	code := C.num_unsealed_bytes(ss.ptr, accessCStr, numBytesPtr)
	if code != 0 {
		return GetNumBytesUnsealedResponse{}, errors.New(errorString(code))
	}

	return GetNumBytesUnsealedResponse{
		NumBytes: numBytes,
	}, nil
}

// destroy deallocates and destroys a DiskBackedSectorStore
func (ss *DiskBackedSectorStore) destroy() error {
	if ss.ptr == nil {
		return errors.New("destroy() - already destroyed")
	}

	// TODO: we need to be absolutely sure that this will not leak memory,
	// specifically the string-fields allocated by Rust (see issue #781)
	C.destroy_storage(ss.ptr)
	ss.ptr = nil

	return nil
}
