package constants

type NetworkType int

const (
	NetworkDefault   NetworkType = 0
	NetworkMainnet   NetworkType = 0x1
	Network2k        NetworkType = 0x2
	NetworkDebug     NetworkType = 0x3
	NetworkCalibnet  NetworkType = 0x4
	NetworkNerpa     NetworkType = 0x5
	NetworkInterop   NetworkType = 0x6
	NetworkForce     NetworkType = 0x7
	NetworkButterfly NetworkType = 0x8

	Integrationnet NetworkType = 0x30
)
