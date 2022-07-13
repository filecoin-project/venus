package market

type PieceStorageInfos struct {
	FsStorage []FsStorage
	S3Storage []S3Storage
}

type FsStorage struct {
	Path     string
	Name     string
	ReadOnly bool
}

type S3Storage struct {
	Name     string
	ReadOnly bool
	EndPoint string
}
