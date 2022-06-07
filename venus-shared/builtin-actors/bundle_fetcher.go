package builtinactors

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("bundle-fetcher")

type BundleFetcher struct {
	path string
}

func NewBundleFetcher(basepath string) (*BundleFetcher, error) {
	path := filepath.Join(basepath, "builtin-actors")
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("error making bundle directory %s: %w", path, err)
	}

	return &BundleFetcher{path: path}, nil
}

func (b *BundleFetcher) FetchFromRelease(version int, release, netw string) (path string, err error) {
	bundleName := fmt.Sprintf("builtin-actors-%s", netw)
	bundleFile := fmt.Sprintf("%s.car", bundleName)
	bundleHash := fmt.Sprintf("%s.sha256", bundleName)
	bundleBasePath := filepath.Join(b.path, fmt.Sprintf("v%d", version), release)

	if err := os.MkdirAll(bundleBasePath, 0755); err != nil {
		return "", fmt.Errorf("error making bundle directory %s: %w", bundleBasePath, err)
	}

	// check if it exists; if it does, check the hash
	bundleFilePath := filepath.Join(bundleBasePath, bundleFile)
	if _, err := os.Stat(bundleFilePath); err == nil {
		err := b.checkRelease(bundleBasePath, bundleFile, bundleHash)
		if err == nil {
			return bundleFilePath, nil
		}

		log.Warnf("invalid bundle %s: %s; refetching", bundleName, err)
	}

	fmt.Println("fetching bundle ", bundleFile)
	if err := b.fetchFromRelease(release, bundleBasePath, bundleFile, bundleHash); err != nil {
		log.Errorf("error fetching bundle %s: %s", bundleName, err)
		return "", fmt.Errorf("error fetching bundle: %w", err)
	}

	if err := b.checkRelease(bundleBasePath, bundleFile, bundleHash); err != nil {
		log.Errorf("error checking bundle %s: %s", bundleName, err)
		return "", fmt.Errorf("error checking bundle: %s", err)
	}

	return bundleFilePath, nil
}

func (b *BundleFetcher) FetchFromURL(version int, release, netw, url, cksum string) (path string, err error) {
	bundleName := fmt.Sprintf("builtin-actors-%s", netw)
	bundleFile := fmt.Sprintf("%s.car", bundleName)
	bundleBasePath := filepath.Join(b.path, fmt.Sprintf("v%d", version), release)

	if err := os.MkdirAll(bundleBasePath, 0755); err != nil {
		return "", fmt.Errorf("error making bundle directory %s: %w", bundleBasePath, err)
	}

	// check if it exists; if it does, check the hash
	bundleFilePath := filepath.Join(bundleBasePath, bundleFile)
	if _, err := os.Stat(bundleFilePath); err == nil {
		err := b.checkHash(bundleBasePath, bundleFile, cksum)
		if err == nil {
			return bundleFilePath, nil
		}

		log.Warnf("invalid bundle %s: %s; refetching", bundleName, err)
	}

	fmt.Println("fetching bundle ", bundleFile)
	if err := b.fetchFromURL(bundleBasePath, bundleFile, url); err != nil {
		log.Errorf("error fetching bundle %s: %s", bundleName, err)
		return "", fmt.Errorf("error fetching bundle: %w", err)
	}

	if err := b.checkHash(bundleBasePath, bundleFile, cksum); err != nil {
		log.Errorf("error checking bundle %s: %s", bundleName, err)
		return "", fmt.Errorf("error checking bundle: %s", err)
	}

	return bundleFilePath, nil
}

func (b *BundleFetcher) fetchURL(url, path string) error {
	fmt.Println("fetching URL: ", url)

	for i := 0; i < 3; i++ {
		resp, err := http.Get(url) //nolint
		if err != nil {
			if isTemporary(err) {
				log.Warnf("temporary error fetching %s: %s; retrying in 1s", url, err)
				time.Sleep(time.Second)
				continue
			}
			return fmt.Errorf("error fetching %s: %w", url, err)
		}
		defer resp.Body.Close() //nolint

		if resp.StatusCode != http.StatusOK {
			log.Warnf("unexpected response fetching %s: %s (%d); retrying in 1s", url, resp.Status, resp.StatusCode)
			time.Sleep(time.Second)
			continue
		}

		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("error opening %s for writing: %w", path, err)
		}
		defer f.Close() //nolint

		if _, err := io.Copy(f, resp.Body); err != nil {
			return fmt.Errorf("error writing %s: %w", path, err)
		}

		return nil
	}

	return fmt.Errorf("all attempts to fetch %s failed", url)
}

func (b *BundleFetcher) fetchFromRelease(release, bundleBasePath, bundleFile, bundleHash string) error {
	bundleHashURL := fmt.Sprintf("https://github.com/filecoin-project/builtin-actors/releases/download/%s/%s",
		release, bundleHash)
	bundleHashPath := filepath.Join(bundleBasePath, bundleHash)
	if err := b.fetchURL(bundleHashURL, bundleHashPath); err != nil {
		return err
	}

	bundleFileURL := fmt.Sprintf("https://github.com/filecoin-project/builtin-actors/releases/download/%s/%s",
		release, bundleFile)
	bundleFilePath := filepath.Join(bundleBasePath, bundleFile)

	return b.fetchURL(bundleFileURL, bundleFilePath)
}

func (b *BundleFetcher) fetchFromURL(bundleBasePath, bundleFile, url string) error {
	bundleFilePath := filepath.Join(bundleBasePath, bundleFile)
	return b.fetchURL(url, bundleFilePath)
}

func (b *BundleFetcher) checkRelease(bundleBasePath, bundleFile, bundleHash string) error {
	bundleHashPath := filepath.Join(bundleBasePath, bundleHash)
	f, err := os.Open(bundleHashPath)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", bundleHashPath, err)
	}
	defer f.Close() //nolint

	bs, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("error reading %s: %w", bundleHashPath, err)
	}

	parts := strings.Split(string(bs), " ")
	hashHex := parts[0]

	return b.checkHash(bundleBasePath, bundleFile, hashHex)
}

func (b *BundleFetcher) checkHash(bundleBasePath, bundleFile, cksum string) error {
	expectedDigest, err := hex.DecodeString(cksum)
	if err != nil {
		return fmt.Errorf("error decoding digest from %s: %w", cksum, err)
	}

	bundleFilePath := filepath.Join(bundleBasePath, bundleFile)
	f, err := os.Open(bundleFilePath)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", bundleFilePath, err)
	}
	defer f.Close() //nolint

	h256 := sha256.New()
	if _, err := io.Copy(h256, f); err != nil {
		return fmt.Errorf("error computing digest for %s: %w", bundleFilePath, err)
	}
	digest := h256.Sum(nil)

	if !bytes.Equal(digest, expectedDigest) {
		return fmt.Errorf("hash mismatch")
	}

	return nil
}

func isTemporary(err error) bool {
	if ne, ok := err.(net.Error); ok {
		return ne.Temporary()
	}

	return false
}
