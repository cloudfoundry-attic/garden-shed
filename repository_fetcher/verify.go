package repository_fetcher

import (
	"errors"
	"io"
	"io/ioutil"
	"os"

	"github.com/docker/distribution/digest"
)

//go:generate counterfeiter . Verifier
type Verifier interface {
	Verify(io.Reader, digest.Digest) (io.ReadCloser, int64, error)
}

var DefaultVerifier = VerifyFunc(Verify)

// Verify reads the given reader in to a temporary file and validates that
// it matches the digest. If it does, it returns a reader for that allows access
// to the data. Otherwise, it returns an error.
// The caller is responsible for closing the returned reader, in order to
// ensure the temporary file is deleted.
func Verify(r io.Reader, d digest.Digest) (io.ReadCloser, int64, error) {
	w, err := digest.NewDigestVerifier(d)
	if err != nil {
		return nil, 0, err
	}

	tmp, err := ioutil.TempFile("", "unverified-layer")
	if err != nil {
		return nil, 0, err
	}

	n, err := io.Copy(io.MultiWriter(w, tmp), r)
	if err != nil {
		return nil, 0, err
	}

	if !w.Verified() {
		return nil, 0, errors.New("digest verification failed")
	}

	_, err = tmp.Seek(0, 0)
	if err != nil {
		return nil, 0, err
	}

	return &deleteCloser{tmp}, n, nil
}

type deleteCloser struct {
	*os.File
}

func (dc *deleteCloser) Close() error {
	return os.Remove(dc.File.Name())
}

type VerifyFunc func(io.Reader, digest.Digest) (io.ReadCloser, int64, error)

func (fn VerifyFunc) Verify(r io.Reader, d digest.Digest) (io.ReadCloser, int64, error) {
	return fn(r, d)
}
