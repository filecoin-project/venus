package fskeystore

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	ci "github.com/libp2p/go-libp2p-core/crypto"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
)

func assertDirContents(dir string, exp []string) error {
	finfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	if len(finfos) != len(exp) {
		return fmt.Errorf("expected %d directory entries", len(exp))
	}

	var names []string
	for _, fi := range finfos {
		names = append(names, fi.Name())
	}

	sort.Strings(names)
	sort.Strings(exp)
	if len(names) != len(exp) {
		return fmt.Errorf("directory had wrong number of entries in it")
	}

	for i, v := range names {
		if v != exp[i] {
			return fmt.Errorf("had wrong entry in directory")
		}
	}
	return nil
}

func TestKeystoreBasics(t *testing.T) {
	tf.UnitTest(t)
	tdir, err := ioutil.TempDir("", "keystore-test")
	if err != nil {
		t.Fatal(err)
	}

	ks, err := NewFSKeystore(tdir)
	if err != nil {
		t.Fatal(err)
	}

	l, err := ks.List()
	if err != nil {
		t.Fatal(err)
	}

	if len(l) != 0 {
		t.Fatal("expected no keys")
	}

	k1 := privKeyOrFatal(t)
	k2 := privKeyOrFatal(t)
	k3 := privKeyOrFatal(t)
	k4 := privKeyOrFatal(t)

	err = ks.Put("foo", k1)
	if err != nil {
		t.Fatal(err)
	}

	err = ks.Put("bar", k2)
	if err != nil {
		t.Fatal(err)
	}

	l, err = ks.List()
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(l)
	if l[0] != "bar" || l[1] != "foo" {
		t.Fatal("wrong entries listed")
	}

	if err := assertDirContents(tdir, []string{"foo", "bar"}); err != nil {
		t.Fatal(err)
	}

	err = ks.Put("foo", k3)
	if err == nil {
		t.Fatal("should not be able to overwrite key")
	}

	if err := assertDirContents(tdir, []string{"foo", "bar"}); err != nil {
		t.Fatal(err)
	}

	exist, err := ks.Has("foo")
	if !exist {
		t.Fatal("should know it has a key named foo")
	}
	if err != nil {
		t.Fatal(err)
	}

	exist, err = ks.Has("nonexistingkey")
	if exist {
		t.Fatal("should know it doesn't have a key named nonexistingkey")
	}
	if err != nil {
		t.Fatal(err)
	}

	if err := ks.Delete("bar"); err != nil {
		t.Fatal(err)
	}

	if err := assertDirContents(tdir, []string{"foo"}); err != nil {
		t.Fatal(err)
	}

	if err := ks.Put("beep", k3); err != nil {
		t.Fatal(err)
	}

	if err := ks.Put("boop", k4); err != nil {
		t.Fatal(err)
	}

	if err := assertDirContents(tdir, []string{"foo", "beep", "boop"}); err != nil {
		t.Fatal(err)
	}

	if err := assertGetKey(ks, "foo", k1); err != nil {
		t.Fatal(err)
	}

	if err := assertGetKey(ks, "beep", k3); err != nil {
		t.Fatal(err)
	}

	if err := assertGetKey(ks, "boop", k4); err != nil {
		t.Fatal(err)
	}

	if err := ks.Put("..///foo/", k1); err == nil {
		t.Fatal("shouldnt be able to put a poorly named key")
	}

	if err := ks.Put("", k1); err == nil {
		t.Fatal("shouldnt be able to put a key with no name")
	}

	if err := ks.Put(".foo", k1); err == nil {
		t.Fatal("shouldnt be able to put a key with a 'hidden' name")
	}
}

func TestInvalidKeyFiles(t *testing.T) {
	tf.UnitTest(t)

	tdir, err := ioutil.TempDir("", "keystore-test")

	if err != nil {
		t.Fatal(err)
	}
	defer assert.NoError(t, os.RemoveAll(tdir))

	ks, err := NewFSKeystore(tdir)
	if err != nil {
		t.Fatal(err)
	}

	bytes := privKeyOrFatal(t)

	err = ioutil.WriteFile(filepath.Join(ks.dir, "valid"), bytes, 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = ioutil.WriteFile(filepath.Join(ks.dir, ".invalid"), bytes, 0644)
	if err != nil {
		t.Fatal(err)
	}

	l, err := ks.List()
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(l)
	if len(l) != 1 {
		t.Fatal("wrong entry count")
	}

	if l[0] != "valid" {
		t.Fatal("wrong entries listed")
	}

	exist, err := ks.Has("valid")
	if err != nil {
		t.Fatal(err)
	}
	if !exist {
		t.Fatal("should know it has a key named valid")
	}

	if _, err = ks.Has(".invalid"); err == nil {
		t.Fatal("shouldnt be able to put a key with a 'hidden' name")
	}
}

func TestNonExistingKey(t *testing.T) {
	tf.UnitTest(t)

	tdir, err := ioutil.TempDir("", "keystore-test")
	if err != nil {
		t.Fatal(err)
	}

	ks, err := NewFSKeystore(tdir)
	if err != nil {
		t.Fatal(err)
	}

	k, err := ks.Get("does-it-exist")
	if err != ErrNoSuchKey {
		t.Fatalf("expected: %s, got %s", ErrNoSuchKey, err)
	}
	if k != nil {
		t.Fatalf("Get on nonexistant key should give nil")
	}
}

func TestMakeKeystoreNoDir(t *testing.T) {
	tf.UnitTest(t)

	_, err := NewFSKeystore("/this/is/not/a/real/dir")
	if err == nil {
		t.Fatal("shouldnt be able to make a keystore in a nonexistant directory")
	}
}

type rr struct{}

func (rr rr) Read(b []byte) (int, error) {
	return rand.Read(b)
}

func privKeyOrFatal(t *testing.T) []byte {
	priv, _, err := ci.GenerateEd25519Key(rr{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := priv.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertGetKey(ks Keystore, name string, exp []byte) error {
	outK, err := ks.Get(name)
	if err != nil {
		return err
	}

	if !bytes.Equal(outK, exp) {
		return fmt.Errorf("key we got out didnt match expectation")
	}

	return nil
}
