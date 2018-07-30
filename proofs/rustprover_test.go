package proofs

import "testing"

func TestRustProverRoundTrip(t *testing.T) {
	p := &RustProver{}

	proverID := make([]uint8, 31)
	for i := 0; i < 31; i++ {
		proverID[i] = uint8(i)
	}

	challengeSeed := make([]uint32, 8)
	for i := 0; i < 8; i++ {
		challengeSeed[i] = uint32(i)
	}

	randomSeed := make([]uint32, 8)
	for i := 7; i >= 0; i-- {
		randomSeed[i] = uint32(i)
	}

	sres := p.Seal(SealRequest{
		challengeSeed: challengeSeed,
		proverID:      proverID,
		randomSeed:    randomSeed,
	})

	if sres.commitments.commR != 12345 {
		t.Fatalf("expected commR to be 12345, but was %d", sres.commitments.commR)
	}

	if sres.commitments.commD != 54321 {
		t.Fatalf("expected commR to be 54321, but was %d", sres.commitments.commD)
	}

	vres := p.VerifySeal(VerifySealRequest{
		commitments: CommitmentPair{
			commR: sres.commitments.commR,
			commD: sres.commitments.commD,
		},
	})

	if !vres {
		t.Fatal("expected VerifySeal(Seal(x)) to be true")
	}
}
