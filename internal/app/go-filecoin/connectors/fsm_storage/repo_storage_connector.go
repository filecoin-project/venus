package fsmstorage

import (
	"errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/vendors/sector-storage/stores"
	"github.com/filecoin-project/lotus/extern/sector-storage/fsutil"
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

func (b *RepoStorageConnector) Stat(path string) (fsutil.FsStat, error) {
	return fsutil.Statfs(path)
}

// returns real disk usage for a file/directory
// os.ErrNotExit when file doesn't exist
func (b *RepoStorageConnector) DiskUsage(path string) (int64, error) {
	si, err := fsutil.FileSize(path)
	if err != nil {
		return 0, err
	}
	return si.OnDisk, nil
}
