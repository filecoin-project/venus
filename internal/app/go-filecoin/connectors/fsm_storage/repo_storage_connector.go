package fsmstorage

import (
	"errors"

	"github.com/filecoin-project/sector-storage/stores"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
)

type RepoStorageConnector struct {
	inner repo.Repo
}

var _ stores.LocalStorage = new(RepoStorageConnector)

func NewRepoStorageConnector(r repo.Repo) *RepoStorageConnector {
	return &RepoStorageConnector{inner: r}
}

func (b *RepoStorageConnector) GetStorage() (stores.StorageConfig, error) {
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

func (b *RepoStorageConnector) SetStorage(f func(*stores.StorageConfig)) error {
	return errors.New("unsupported operation: manipulating store paths must happen through go-filecoin")
}
