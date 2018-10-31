package sectorbuilder

import "testing"

func TestRustSectorBuilderLifecycle(t *testing.T) {
	var array [31]byte
	copy(array[:], requireRandomBytes(t, 31))

	sb := NewRustSectorBuilder(123, "metadata", array, "staged", "sealed")
	sb.printSectorBuilderState()
}
