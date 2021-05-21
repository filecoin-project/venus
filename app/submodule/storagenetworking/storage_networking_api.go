package storagenetworking

type IStorageNetworking interface {
}

var _ IStorageNetworking = &storageNetworkingAPI{}

type storageNetworkingAPI struct { //nolint
	storageNetworking *StorageNetworkingSubmodule
}
