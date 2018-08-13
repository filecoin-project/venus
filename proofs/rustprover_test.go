package proofs

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

func TestRustProverRoundTrip(t *testing.T) {
	p := &RustProver{}

	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("error creating temp (input) file %s", err)
	}
	defer os.Remove(tmpFile.Name())

	srcPath := tmpFile.Name()
	dstPath := fmt.Sprintf("%s42", srcPath)

	proverID := make([]uint8, 31)
	for i := 0; i < 31; i++ {
		proverID[i] = uint8(i)
	}

	challengeSeed := make([]uint8, 32)
	for i := 0; i < 32; i++ {
		challengeSeed[i] = uint8(i)
	}

	randomSeed := make([]uint8, 32)
	for i := 31; i >= 0; i-- {
		randomSeed[i] = uint8(i)
	}

	sres := p.Seal(SealRequest{
		UnsealedPath:  srcPath,
		SealedPath:    dstPath,
		ChallengeSeed: challengeSeed,
		ProverID:      proverID,
		RandomSeed:    randomSeed,
	})

	_, err = os.Stat(dstPath)
	if err != nil {
		t.Fatalf("Seal() operation didn't create sealed sector-file %s", dstPath)
	}

	expected := "12345678901234567890123456789012"
	if string(sres.Commitments.CommR) != expected {
		t.Fatalf("expected CommR to be %s, but was %s", expected, sres.Commitments.CommR)
	}

	expected = "09876543210987654321098765432109"
	if string(sres.Commitments.CommD) != expected {
		t.Fatalf("expected CommD to be %s, but was %s", expected, sres.Commitments.CommD)
	}

	vres := p.VerifySeal(VerifySealRequest{
		Commitments: CommitmentPair{
			CommR: sres.Commitments.CommR,
			CommD: sres.Commitments.CommD,
		},
	})

	if !vres {
		t.Fatal("expected VerifySeal(Seal(x)) to be true")
	}
}
