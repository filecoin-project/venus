package drgporep

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"testing"
	"time"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-proofs/crypto"
	"github.com/filecoin-project/go-proofs/porep/drgporep/drgraph"
)

type Context struct {
	NodeSize int
	Graph    *drgraph.Graph
	Prover   *Prover
	Verifier *Verifier
	CommR    []byte
	Dir      string
}

// func TestBlockchainSetting (t *testing.T) {
// 	// Before filecoin starts a DRG graph is chosen
// 	nodeSize := 16 // amount of bytes stored at each node
// 	n := len(data) / nodeSize // number of nodes in a graph
// 	G := drgraph.New(n)	// generate the graph
// 	// ...
// 	// client and storage miner make a deal
// 	miner := NewProver("miner11111111111", G, nodeSize)

// }

type ReadWriterFile struct {
	File *os.File
}

func WrapFile(f *os.File) *ReadWriterFile {
	return &ReadWriterFile{
		File: f,
	}
}

func (r *ReadWriterFile) Size() int64 {
	info, err := r.File.Stat()
	if err != nil {
		panic(err)
	}

	s := info.Size()
	return s
}

// Resize pads the given file to the given target size.
func Resize(f *os.File, target int64) {
	info, err := f.Stat()
	if err != nil {
		panic(err)
	}
	currentSize := info.Size()
	if currentSize >= target {
		return
	}

	_, err = f.WriteAt(make([]byte, target-currentSize), currentSize)
	if err != nil {
		panic(err)
	}
}

func (r *ReadWriterFile) DataAt(offset, length uint64, cb func([]byte) error) error {
	b := make([]byte, length)
	if int64(offset)+int64(length) > r.Size() {
		return fmt.Errorf("out of bounds: offset=%d, len=%d, size=%d ", offset, length, r.Size())
	}
	if _, err := r.File.ReadAt(b, int64(offset)); err != nil {
		return errors.Wrapf(err, "failed to read (offset=%d, len=%d)", offset, length)
	}

	var out []byte
	if uint64(r.Size()) < length {
		out = make([]byte, length)
		copy(out, b)
	} else {
		out = b
	}

	if err := cb(out); err != nil {
		return errors.Wrap(err, "cb failed")
	}

	if _, err := r.File.WriteAt(out, int64(offset)); err != nil {
		return errors.Wrap(err, "failed to write")
	}

	return nil
}

// - Add back params for input size
// - validate input size is multiple of nodeSize
// -

func RunSetup(t *testing.T, nodeSize int, data []byte) Context {
	t.Helper()

	// Public params
	n := len(data) / nodeSize
	G := drgraph.NewBucketSample(n, nodeSize)

	// Setup files

	// Directory that holds all our files
	dir, err := ioutil.TempDir("", "seal")
	if err != nil {
		t.Fatal(err)
	}

	// Input file that will host the actual input data
	inFile, err := ioutil.TempFile(dir, "in")
	if err != nil {
		t.Fatal(err)
	}

	// write our data into the file
	if _, err := inFile.Write(data); err != nil {
		t.Fatal(err)
	}

	// Need to reset after the write, to ensure follow on reads start at
	// the beginning of the data
	if _, err := inFile.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}

	Resize(inFile, int64(G.ExpectedSize(nodeSize)))

	// Verifier - setup
	veri := NewVerifier(G, nodeSize)
	if err := veri.Setup(WrapFile(inFile)); err != nil {
		t.Fatal(err)
	}

	// Prover - setup
	prov := NewProver("miner11111111111", G, nodeSize)

	// need to reset after using inFile in veri.Setup
	if _, err := inFile.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}

	inWFile := WrapFile(inFile)
	commR, err := prov.Setup(inWFile)
	if err != nil {
		t.Fatal(err)
	}

	// Prover sends setup to Verifier
	// TODO(@ben): do we need this?
	return Context{nodeSize, G, prov, veri, commR, dir}
}

func RunAllInteractions(context Context) (bool, error) {
	// clean up the directory after we are done with our work
	defer os.RemoveAll(context.Dir)

	for challenge := 0; challenge < context.Graph.Nodes; challenge++ {
		res, err := RunInteraction(context, challenge)
		if err != nil {
			return res, err
		}
		if !res {
			panic("false proof must return error")
		}
	}
	return true, nil
}

func RunInteraction(Context Context, challenge int) (bool, error) {
	proof := Context.Prover.Prove(challenge)
	res, err := Context.Verifier.Verify(challenge, Context.Prover.ID, Context.CommR, proof)

	if err != nil {
		fmt.Println("\nProver setup:")

		var ciphertexts []byte
		for _, pred := range Context.Prover.Graph.Pred[challenge+1] {
			d, err := crypto.DataAtNode(Context.Prover.Replica, pred, Context.NodeSize)
			if err != nil {
				return false, err
			}
			ciphertexts = append(ciphertexts, d...)
		}
		fmt.Println("\tchall:\t", challenge)
		fmt.Println("\tparents:", ciphertexts)
		fmt.Println("\tkey:\t", crypto.Kdf(ciphertexts))
		trepl, err := crypto.DataAtNode(Context.Prover.Replica, challenge+1, Context.NodeSize)
		if err != nil {
			return false, err
		}
		node := Context.Prover.TreeD.Nodes[challenge+1]
		if err != nil {
			return false, err
		}

		fmt.Println("\trepl:\t", trepl)
		fmt.Println("\tmerkle node:\t", node)

		fmt.Println("challenge", challenge)
		fmt.Println("Prover proof:")
		fmt.Println("\trepl:\t", proof.ReplicaNode.Data)

		fmt.Println("Verifier verify:")
		var ciphertextsV []byte
		for _, mProof := range proof.ReplicaParents {
			ciphertextsV = append(ciphertextsV, mProof.Data...)
		}
		fmt.Println("\tparents:", ciphertextsV)
		keyV := crypto.Kdf(ciphertextsV)
		fmt.Println("\tkey:\t", keyV)
		fmt.Println("\tGOT:\t", crypto.Dec(Context.Prover.ID, keyV, proof.ReplicaNode.Data))
	}

	return res, err
}

func randomData(size uint) []byte {
	out := make([]byte, size)
	// rand.Seed(seed)
	_, err := rand.Read(out)
	if err != nil {
		panic(err)
	}
	return out
}

func TestSealUnseal(t *testing.T) {
	// Number of nodes in the graph
	n := 3
	// Amount of data to encode at each node
	nodeSize := 16
	// Data to be sealed

	dir, err := ioutil.TempDir("", "seal")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up

	inFile, err := ioutil.TempFile(dir, "in")
	if err != nil {
		t.Fatal(err)
	}

	raw := []byte("111111111111111122222222222222223333333333333333")
	if _, err := inFile.Write(raw); err != nil {
		t.Fatal(err)
	}

	if _, err := inFile.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}

	// Generate a graph
	G := drgraph.NewBucketSample(n, nodeSize)

	// Our sealer
	s := NewProver("miner11111111111", G, nodeSize)

	inWFile := WrapFile(inFile)
	Resize(inFile, int64(G.ExpectedSize(nodeSize)))

	// Seal
	s.seal(inWFile)

	decFile, err := ioutil.TempFile(dir, "dec")
	if err != nil {
		t.Fatal(err)
	}
	Resize(decFile, int64(G.ExpectedSize(nodeSize)))

	s.Replica = inWFile
	s.Unseal(0, WrapFile(decFile))

	unseal, err := ioutil.ReadFile(decFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(raw, unseal) {
		t.Fatalf("Expected\n%x\nto equal\n%x", raw, unseal)
	}

	// check that coded equals to unseal
	// for i := range raw {
	// 	if raw[i] != unseal[i] {
	// 		t.Fatal("Byte at index", i, "is", unseal[i], "instead of", raw[i])
	// 	}
	// }
}

// make sure things don't get optimized away
var unseal []byte

func benchmarkUnseal(n, nodeSize int, par float64, b *testing.B) {
	// Data to be sealed
	raw := randomData(1024 * 1024)

	// Generate a graph
	G := drgraph.NewBucketSample(n, nodeSize)

	dir, err := ioutil.TempDir("", "seal")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	inFile, err := ioutil.TempFile(dir, "in")
	if err != nil {
		b.Fatal(err)
	}

	if _, err := inFile.Write(raw); err != nil {
		b.Fatal(err)
	}

	if _, err := inFile.Seek(0, io.SeekStart); err != nil {
		b.Fatal(err)
	}
	inWFile := WrapFile(inFile)

	// set the size of the file
	Resize(inFile, int64(G.ExpectedSize(nodeSize)))

	decFile, err := ioutil.TempFile(dir, "dec")
	if err != nil {
		b.Fatal(err)
	}

	// set the size of the file
	Resize(decFile, int64(G.ExpectedSize(nodeSize)))

	// Our sealer
	s := NewProver("miner11111111111", G, nodeSize)

	// Seal
	s.seal(inWFile)

	b.ResetTimer()

	s.Replica = inWFile

	for i := 0; i < b.N; i++ {
		// Test length of unseal
		s.Unseal(par, WrapFile(decFile))
	}
}

// Parallel, num cpus
func BenchmarkUnsealParN3NodeSize16(b *testing.B)     { benchmarkUnseal(3, 16, 0, b) }
func BenchmarkUnsealParN30NodeSize16(b *testing.B)    { benchmarkUnseal(30, 16, 0, b) }
func BenchmarkUnsealParN300NodeSize16(b *testing.B)   { benchmarkUnseal(300, 16, 0, b) }
func BenchmarkUnsealParN10000NodeSize16(b *testing.B) { benchmarkUnseal(10000, 16, 0, b) }

// single threaded
func BenchmarkUnsealSingleN3NodeSize16(b *testing.B)     { benchmarkUnseal(3, 16, 1, b) }
func BenchmarkUnsealSingleN30NodeSize16(b *testing.B)    { benchmarkUnseal(30, 16, 1, b) }
func BenchmarkUnsealSingleN300NodeSize16(b *testing.B)   { benchmarkUnseal(300, 16, 1, b) }
func BenchmarkUnsealSingleN10000NodeSize16(b *testing.B) { benchmarkUnseal(10000, 16, 1, b) }

func Test3Nodes(t *testing.T) {
	context := RunSetup(t, 16, []byte("AAAAAAAAAAAAAAAA9999999999999999ZZZZZZZZZZZZZZZZ"))
	res, err := RunAllInteractions(context)

	if err != nil {
		t.Fatal(err)
	}
	if !res {
		t.Fatal("proof failed")
	}
}

func Test100Nodes(t *testing.T) {
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	fmt.Println("graph seed:", seed)

	nodeSize := 16
	data := randomData(uint(nodeSize) * 100)
	context := RunSetup(t, nodeSize, data)
	res, err := RunAllInteractions(context)

	if err != nil {
		t.Fatal(err)
	}
	if !res {
		t.Fatal("proof failed")
	}
}

func Test300Nodes(t *testing.T) {
	nodeSize := 16
	data := randomData(uint(nodeSize) * 300)
	context := RunSetup(t, nodeSize, data)
	res, err := RunAllInteractions(context)

	if err != nil {
		t.Fatal(err)
	}
	if !res {
		t.Fatal("proof failed")
	}
}

func TestDRGBucket(t *testing.T) {
	n := 10
	m := 5
	gDash := drgraph.NewDRSample(n * m)
	g := drgraph.NewBucketSample(n, m)
	for j := 1; j <= g.Nodes; j++ {
		for _, i := range g.Pred[j] {
			if i == j {
				t.Fatal("some nodes link to themselves")
			}
			if i < j {
				t.Fatal("graph is not a dag", i, j)
			}
		}
	}
	fmt.Println("g':", gDash)
	fmt.Println("g :", g)
}

func TestAES(t *testing.T) {
	key := []byte("12345678123456781234567812345678")
	plaintext := []byte("abcdefghijklmnopabcdefghijklmnoq")

	for i := 0; i < 100; i++ {
		_, _ = rand.Read(key)
		_, _ = rand.Read(plaintext)

		if len(key) != 32 || len(plaintext) != 32 {
			t.Fatal("key or plaintext is of a wrong size")
		}
		ciphertext := crypto.AESEnc(key, plaintext)
		res := crypto.AESDec(key, ciphertext)

		if !bytes.Equal(res, plaintext) {
			fmt.Println("\nkey  ", (&big.Int{}).SetBytes(key), len(key))
			fmt.Println("plain", plaintext, len(plaintext))
			fmt.Println("cypht", ciphertext, len(ciphertext))
			fmt.Println("decrt", res, len(res))

			t.Fatalf("Expected %x to equal %x", res, plaintext)
		}
	}
}

func SkipTestSloth(t *testing.T) {
	p, v := &big.Int{}, &big.Int{}
	p.SetString("135741874269561010210788515394321418560783524050838812444665528300130001644649", 10)
	v.SetString("90494582846374006807192343596214279040522349367225874963110352200086667763099", 10)

	key := []byte("12345678123456781234567812345678")
	plaintext := []byte("abcdefghijklmnopabcdefghijklmnop")

	for i := 0; i < 4; i++ {
		_, _ = rand.Read(key)
		_, _ = rand.Read(plaintext)

		ciphertext := crypto.SlothEnc(key, plaintext, p, v)
		res := crypto.SlothDec(key, ciphertext, p)

		if !bytes.Equal(res, plaintext) {
			fmt.Println("\nkey  ", (&big.Int{}).SetBytes(key), len(key))
			fmt.Println("plain", (&big.Int{}).SetBytes(plaintext), len(plaintext))
			fmt.Println("cypht", (&big.Int{}).SetBytes(ciphertext), len(ciphertext))
			fmt.Println("decrt", (&big.Int{}).SetBytes(res), len(res))
			t.Fatalf("Expected %x to equal %x", (&big.Int{}).SetBytes(res), (&big.Int{}).SetBytes(plaintext))
		}
	}
}

func SkipTestMiMC(t *testing.T) {
	p, v := &big.Int{}, &big.Int{}
	p.SetString("135741874269561010210788515394321418560783524050838812444665528300130001644649", 10)
	v.SetString("90494582846374006807192343596214279040522349367225874963110352200086667763099", 10)

	key := []byte("12345678123456781234567812345678")
	plaintext := []byte("abcdefghijklmnopabcdefghijklmnop")

	for i := 0; i < 100; i++ {
		_, _ = rand.Read(key)
		_, _ = rand.Read(plaintext)

		ciphertext := crypto.MiMCEnc(key, plaintext, p, v)
		res := crypto.MiMCDec(key, ciphertext, p)

		if !bytes.Equal(res, plaintext) {
			fmt.Println("\nkey  ", (&big.Int{}).SetBytes(key), len(key))
			fmt.Println("plain", (&big.Int{}).SetBytes(plaintext), len(plaintext))
			fmt.Println("cypht", (&big.Int{}).SetBytes(ciphertext), len(ciphertext))
			fmt.Println("decrt", (&big.Int{}).SetBytes(res), len(res))
			t.Fatalf("Expected %x to equal %x", (&big.Int{}).SetBytes(res), (&big.Int{}).SetBytes(plaintext))
		}
	}
}

func TestMiMCEdgeTest(t *testing.T) {
	p, v, _key, _plaintext := &big.Int{}, &big.Int{}, &big.Int{}, &big.Int{}
	p.SetString("135741874269561010210788515394321418560783524050838812444665528300130001644649", 10)
	v.SetString("90494582846374006807192343596214279040522349367225874963110352200086667763099", 10)

	_key.SetString("48836180461970794677231737222049675332623260768136628466721098606477997272991", 10)
	_plaintext.SetString("115356136812802156427691759386677700174352153452394142809491974856892118717494", 10)

	key := _key.Bytes()
	plaintext := _plaintext.Bytes()

	ciphertext := crypto.MiMCEnc(key, plaintext, p, v)
	res := crypto.MiMCDec(key, ciphertext, p)

	if !bytes.Equal(res, plaintext) {
		fmt.Println("\nkey  ", (&big.Int{}).SetBytes(key), len(key))
		fmt.Println("plain", (&big.Int{}).SetBytes(plaintext), len(plaintext))
		fmt.Println("cypht", (&big.Int{}).SetBytes(ciphertext), len(ciphertext))
		fmt.Println("decrt", (&big.Int{}).SetBytes(res), len(res))
		t.Fatalf("Expected %x to equal %x", (&big.Int{}).SetBytes(res), (&big.Int{}).SetBytes(plaintext))
	}
}

func TestIterativeAES(t *testing.T) {
	key := []byte("12345678123456781234567812345678")
	plaintext := []byte("abcdefghijklmnopabcdefghijklmnop")

	for i := 0; i < 100; i++ {
		_, _ = rand.Read(key)
		_, _ = rand.Read(plaintext)

		// Try with AES
		AESCiphertext := crypto.AESEnc(key, plaintext)
		AESCiphertext = crypto.AESEnc(key, AESCiphertext)
		AESPlaintext := crypto.AESDec(key, AESCiphertext)
		AESPlaintext = crypto.AESDec(key, AESPlaintext)

		if !bytes.Equal(AESPlaintext, plaintext) {
			fmt.Println("\nkey  ", (&big.Int{}).SetBytes(key), len(key))
			fmt.Println("plain", (&big.Int{}).SetBytes(plaintext), len(plaintext))
			fmt.Println("cypht", (&big.Int{}).SetBytes(AESCiphertext), len(AESCiphertext))
			fmt.Println("decrt", (&big.Int{}).SetBytes(AESPlaintext), len(AESPlaintext))
			t.Fatalf("Expected %x to equal %x", (&big.Int{}).SetBytes(AESPlaintext), (&big.Int{}).SetBytes(plaintext))
		}
	}
}

func SkipTestIterativeMiMC(t *testing.T) {
	p, v := &big.Int{}, &big.Int{}
	p.SetString("135741874269561010210788515394321418560783524050838812444665528300130001644649", 10)
	v.SetString("90494582846374006807192343596214279040522349367225874963110352200086667763099", 10)

	key := []byte("12345678123456781234567812345678")
	plaintext := []byte("abcdefghijklmnopabcdefghijklmnop")

	for i := 0; i < 100; i++ {
		_, _ = rand.Read(key)
		_, _ = rand.Read(plaintext)

		// Try with MiMC
		MiMCCiphertext := crypto.MiMCEnc(key, plaintext, p, v)
		MiMCCiphertext = crypto.MiMCEnc(key, MiMCCiphertext, p, v)
		MiMCPlaintext := crypto.MiMCDec(key, MiMCCiphertext, p)
		MiMCPlaintext = crypto.MiMCDec(key, MiMCPlaintext, p)

		if !bytes.Equal(MiMCPlaintext, plaintext) {
			fmt.Println("\nkey  ", (&big.Int{}).SetBytes(key), len(key))
			fmt.Println("plain", (&big.Int{}).SetBytes(plaintext), len(plaintext))
			fmt.Println("cypht", (&big.Int{}).SetBytes(MiMCCiphertext), len(MiMCCiphertext))
			fmt.Println("decrt", (&big.Int{}).SetBytes(MiMCPlaintext), len(MiMCPlaintext))
			t.Fatalf("Expected %x to equal %x", (&big.Int{}).SetBytes(MiMCPlaintext), (&big.Int{}).SetBytes(plaintext))
		}
	}
}

func SkipTestIterativeSloth(t *testing.T) {
	p, v := &big.Int{}, &big.Int{}
	p.SetString("135741874269561010210788515394321418560783524050838812444665528300130001644649", 10)
	v.SetString("90494582846374006807192343596214279040522349367225874963110352200086667763099", 10)

	key := []byte("12345678123456781234567812345678")
	plaintext := []byte("abcdefghijklmnopabcdefghijklmnop")

	// Try with Sloth
	slothCiphertext := crypto.SlothEnc(key, plaintext, p, v)
	slothCiphertext = crypto.SlothEnc(key, slothCiphertext, p, v)
	slothPlaintext := crypto.SlothDec(key, slothCiphertext, p)
	slothPlaintext = crypto.SlothDec(key, slothPlaintext, p)

	if !bytes.Equal(slothPlaintext, plaintext) {
		t.Fatalf("Sloth Expected %x to equal %x", slothPlaintext, plaintext)
	}
}

func SkipTestSlothIterative(t *testing.T) {
	p, v := &big.Int{}, &big.Int{}
	p.SetString("135741874269561010210788515394321418560783524050838812444665528300130001644649", 10)
	v.SetString("90494582846374006807192343596214279040522349367225874963110352200086667763099", 10)

	key := []byte("12345678123456781234567812345678")
	plaintext := []byte("abcdefghijklmnopabcdefghijklmnop")
	ciphertext := crypto.SlothIterativeEnc(key, plaintext, p, v, 2)
	res := crypto.SlothIterativeDec(key, ciphertext, p, 2)

	if !bytes.Equal(res, plaintext) {
		fmt.Println("\nkey  ", (&big.Int{}).SetBytes(key), len(key))
		fmt.Println("plain", (&big.Int{}).SetBytes(plaintext), len(plaintext))
		fmt.Println("cypht", (&big.Int{}).SetBytes(ciphertext), len(ciphertext))
		fmt.Println("decrt", (&big.Int{}).SetBytes(res), len(res))
		t.Fatalf("Expected %x to equal %x", (&big.Int{}).SetBytes(res), (&big.Int{}).SetBytes(plaintext))
	}
}
