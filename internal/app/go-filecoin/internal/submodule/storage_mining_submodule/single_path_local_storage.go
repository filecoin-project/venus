package storage_mining_submodule

import (
	"golang.org/x/xerrors"

	"github.com/filecoin-project/sector-storage/stores"
)

type singlePathLocalStorage struct {
	sectorDirPath string
}

var _ stores.LocalStorage = new(singlePathLocalStorage)

func newSinglePathLocalStorage(sectorDirPath string) *singlePathLocalStorage {
	return &singlePathLocalStorage{sectorDirPath: sectorDirPath}
}

func (b *singlePathLocalStorage) GetStorage() (stores.StorageConfig, error) {
	return stores.StorageConfig{StoragePaths: []stores.LocalPath{{
		Path: b.sectorDirPath,
	}}}, nil
}

func (b *singlePathLocalStorage) SetStorage(func(*stores.StorageConfig)) error {
	return xerrors.New("unsupported operation")
}
