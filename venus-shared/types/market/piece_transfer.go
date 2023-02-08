package market

const (
	PiecesTransferS3 = "s3"
	PiecesTransferFs = "fs"
)

type FsTransfer struct {
	Path string
}

type S3Transfer struct {
	EndPoint string
	Bucket   string
	SubDir   string

	AccessKey string
	SecretKey string
	Token     string

	Key string
}

// Transfer has the parameters for a data transfer
type Transfer struct {
	// The type of transfer eg "http"
	Type string
	// A byte array containing marshalled data specific to the transfer type
	// eg a JSON encoded struct { URL: "<url>", Headers: {...} }
	Params []byte
}
