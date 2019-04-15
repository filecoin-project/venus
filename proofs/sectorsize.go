package proofs

// #cgo LDFLAGS: -L${SRCDIR}/../proofs/lib -lfilecoin_proofs
// #cgo pkg-config: ${SRCDIR}/../proofs/lib/pkgconfig/libfilecoin_proofs.pc
// #include "../proofs/include/libfilecoin_proofs.h"
import "C"
import (
	"unsafe"
)

func SectorSize(ssType Mode) uint64 {
	scfg, err := CProofsMode(ssType)
	if err != nil {
		return 0
	}

	numFromC := C.get_max_user_bytes_per_staged_sector((*C.ConfiguredStore)(unsafe.Pointer(scfg)))
	return uint64(numFromC)
}
