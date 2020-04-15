package storage_mining_submodule

import (
	"errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/sector-storage/stores"
)

type repoStorageConnector struct {
	inner repo.Repo
}

var _ stores.LocalStorage = new(repoStorageConnector)

func newRepoStorageConnector(r repo.Repo) *repoStorageConnector {
	return &repoStorageConnector{inner: r}
}

func (b *repoStorageConnector) GetStorage() (stores.StorageConfig, error) {
	rpt, err := b.inner.Path()
	if err != nil {
		return stores.StorageConfig{}, err
	}

	scg := b.inner.Config().SectorBase

	spt, err := paths.GetSectorPath(scg.RootDirPath, rpt)
	if err != nil {
		return stores.StorageConfig{}, err
	}

	out := stores.StorageConfig{StoragePaths: []stores.LocalPath{{Path: spt}}}

	if scg.PreSealedSectorsDirPath != "" {
		out.StoragePaths = append(out.StoragePaths, stores.LocalPath{Path: scg.PreSealedSectorsDirPath})
	}

	return out, nil
}

func (b *repoStorageConnector) SetStorage(f func(*stores.StorageConfig)) error {
	return errors.New("unsupported operation: manipulating store paths must happen through go-filecoin")
}
