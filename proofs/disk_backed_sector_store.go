package proofs

import (
	"unsafe"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
)

/*
#cgo LDFLAGS: -L${SRCDIR}/rust-proofs/target/release -Wl,-rpath,${SRCDIR}/rust-proofs/target/release/ -lproofs
#include "./rust-proofs/libproofs.h"

*/
import "C"

// DiskBackedSectorStore is a struct which serves as a proxy for a DiskBackedSectorStore in Rust
type DiskBackedSectorStore struct {
	ptr *C.DiskBackedStorage
}

var _ SectorStore = &DiskBackedSectorStore{}

// NewDiskBackedSectorStore allocates and returns a new DiskBackedSectorStore
func NewDiskBackedSectorStore(staging string, sealed string) *DiskBackedSectorStore {
	stagingDirPath := C.CString(staging)
	defer C.free(unsafe.Pointer(stagingDirPath))

	sealedDirPath := C.CString(sealed)
	defer C.free(unsafe.Pointer(sealedDirPath))

	ptr := C.init_disk_backed_storage(stagingDirPath, sealedDirPath)

	return &DiskBackedSectorStore{
		ptr: ptr,
	}
}

// NewSealedSectorAccess dispenses a new sealed sector access
func (ss *DiskBackedSectorStore) NewSealedSectorAccess() (string, error) {
	if ss.ptr == nil {
		return "", errors.New("NewSealedSectorAccess() - ss.ptr is nil")
	}

	p := C.new_sealed_sector_access(ss.ptr)
	defer C.free(unsafe.Pointer(p))

	return C.GoString(p), nil
}

// NewStagingSectorAccess dispenses a new staging sector access
func (ss *DiskBackedSectorStore) NewStagingSectorAccess() (string, error) {
	if ss.ptr == nil {
		return "", errors.New("NewStagingSectorAccess() - ss.ptr is nil")
	}

	p := C.new_staging_sector_access(ss.ptr)
	defer C.free(unsafe.Pointer(p))

	return C.GoString(p), nil
}

// destroy deallocates and destroys a DiskBackedSectorStore
func (ss *DiskBackedSectorStore) destroy() error {
	if ss.ptr == nil {
		return errors.New("destroy() - already destroyed")
	}

	// TODO: we need to be absolutely sure that this will not leak memory,
	// specifically the string-fields allocated by Rust (see issue #781)
	C.destroy_disk_backed_storage(ss.ptr)
	ss.ptr = nil

	return nil
}
