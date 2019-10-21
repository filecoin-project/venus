package gen

import (
	"fmt"
	"io"
)

// IpldCborTypeEncodingGenerator generates encoding/decoding implementations for the IpldCbor encoding.
//
// This code generator only with the IpldCborEncoder/IpldCborDecoder pair
// and is intended as an intermediate step towards a pure CBOR encoder/decoder.
type IpldCborTypeEncodingGenerator struct {
}

// WriteImports outputs the imports.
func (generator IpldCborTypeEncodingGenerator) WriteImports(w io.Writer) error {
	return doTemplate(w, nil, `
import (
	"github.com/filecoin-project/go-filecoin/encoding"
)
`)
}

// WriteInit outputs the init for the file.
func (generator IpldCborTypeEncodingGenerator) WriteInit(w io.Writer, tis []TypeInfo) error {
	if _, err := fmt.Fprintln(w, "\nfunc init() {"); err != nil {
		return err
	}

	for _, ti := range tis {
		if _, err := fmt.Fprintf(w, "    encoding.RegisterIpldCborType(%s{})\n", ti.Name); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, "}"); err != nil {
		return err
	}

	return nil
}

// WriteEncodingForType outputs the encoding for the given type.
func (generator IpldCborTypeEncodingGenerator) WriteEncodingForType(w io.Writer, ti TypeInfo) error {
	if err := doTemplate(w, ti, `
//
// Encoding/Decoding impls for {{ .Name }} 
// 
	`),  err != nil {
		return err
	}

	if err := writeEncode(w, ti); err != nil {
		return err
	}

	if err := writeDecode(w, ti); err != nil {
		return err
	}

	return nil
}

func writeEncode(w io.Writer, ti TypeInfo) error {
	return doTemplate(w, ti, `
func (p {{ .Name }}) Encode(encoder encoding.Encoder) error {
	var err error

	if err = encoder.EncodeObject(p); err != nil {
		return err
	}

	return nil
}
	`)
}

func writeDecode(w io.Writer, ti TypeInfo) error {
	return doTemplate(w, ti, `
func (p *{{ .Name }}) Decode(decoder encoding.Decoder) error {
	if err := decoder.DecodeObject(p); err != nil {
		return err
	}

	return nil
}	
`)
}
