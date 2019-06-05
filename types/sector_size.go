package types

// OneKiBSectorSize contain 1024 bytes after sealing. These sectors are used
// when the network is in TestProofsMode.
var OneKiBSectorSize = NewBytesAmount(1024)

// TwoHundredFiftySixMiBSectorSize contain 256MiB after sealing. These sectors
// are used when the network is in LiveProofsMode.
var TwoHundredFiftySixMiBSectorSize = NewBytesAmount(1 << 28)
