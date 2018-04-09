package repository_fetcher

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/docker/docker/image"

	"github.com/docker/distribution/digest"

	"code.cloudfoundry.org/garden-shed/distclient"
	"code.cloudfoundry.org/garden-shed/layercake"
	"code.cloudfoundry.org/garden-shed/quotaedreader"
	"code.cloudfoundry.org/lager"
)

type Remote struct {
	DefaultHost string
	Dial        Dialer
	Cake        layercake.Cake
	Verifier    Verifier

	FetchLock *FetchLock
}

func NewRemote(defaultHost string, cake layercake.Cake, dialer Dialer, verifier Verifier) *Remote {
	return &Remote{
		DefaultHost: defaultHost,
		Dial:        dialer,
		Cake:        cake,
		Verifier:    verifier,
		FetchLock:   NewFetchLock(),
	}
}

func (r *Remote) Fetch(log lager.Logger, u *url.URL, username, password string, diskQuota int64) (*Image, error) {
	log = log.Session("fetch", lager.Data{"url": u})

	log.Info("start")
	defer log.Info("finished")

	conn, manifest, err := r.manifest(log, u, username, password)
	if err != nil {
		return nil, err
	}

	totalImageSize := int64(0)
	for _, layer := range manifest.Layers {
		totalImageSize += layer.Image.Size
	}

	if diskQuota > 0 && totalImageSize > diskQuota {
		return nil, errors.New("quota exceeded")
	}

	var env []string
	var vols []string
	remainingQuota := diskQuota
	if diskQuota <= 0 {
		remainingQuota = -1
	}
	for _, layer := range manifest.Layers {
		if layer.Image.Config != nil {
			env = append(env, layer.Image.Config.Env...)
			vols = append(vols, keys(layer.Image.Config.Volumes)...)
		}

		size, err := r.fetchLayer(log, conn, layer, remainingQuota)
		if err != nil {
			return nil, err
		}
		remainingQuota -= size
	}

	return &Image{
		ImageID: hex(manifest.Layers[len(manifest.Layers)-1].StrongID),
		Env:     env,
		Volumes: vols,
		Size:    totalImageSize,
	}, nil
}

func (r *Remote) FetchID(log lager.Logger, u *url.URL) (layercake.ID, error) {
	_, manifest, err := r.manifest(log.Session("fetch-id"), u, "", "")
	if err != nil {
		return nil, err
	}

	return layercake.DockerImageID(hex(manifest.Layers[len(manifest.Layers)-1].StrongID)), nil
}

func (r *Remote) manifest(log lager.Logger, u *url.URL, username, password string) (distclient.Conn, *distclient.Manifest, error) {
	log = log.Session("get-manifest", lager.Data{"url": u})

	log.Debug("started")
	defer log.Debug("got")

	host := u.Host
	if host == "" {
		host = r.DefaultHost
	}

	isDockerHub := host == "registry-1.docker.io"
	path := u.Path[1:] // strip off initial '/'
	isOfficialImage := strings.Index(path, "/") < 0
	if isDockerHub && isOfficialImage {
		// The Docker Hub keeps manifests of official images under library/
		path = "library/" + path
	}

	tag := u.Fragment
	if tag == "" {
		tag = "latest"
	}

	conn, err := r.Dial.Dial(log, host, path, username, password)
	if err != nil {
		return nil, nil, err
	}

	manifest, err := conn.GetManifest(log, tag)
	if err != nil {
		return nil, nil, fmt.Errorf("get manifest for tag %s on repo %s: %s", u.Fragment, u, err)
	}

	return conn, manifest, err
}

func (r *Remote) fetchLayer(log lager.Logger, conn distclient.Conn, layer distclient.Layer, quota int64) (int64, error) {
	log = log.Session("fetch-layer", lager.Data{"size": layer.Image.Size, "blobsum": layer.BlobSum, "id": layer.StrongID, "parent": layer.ParentStrongID})

	log.Info("start")
	defer log.Info("fetched")

	r.FetchLock.Acquire(layer.BlobSum.String())
	defer r.FetchLock.Release(layer.BlobSum.String())

	if image, err := r.Cake.Get(layercake.DockerImageID(hex(layer.StrongID))); err == nil {
		log.Info("got-cache")
		return image.Size, nil
	}

	blob, err := conn.GetBlobReader(log, layer.BlobSum)
	if err != nil {
		return 0, err
	}

	blob = applyQuota(blob, quota)

	log.Debug("verifying")
	verifiedBlob, size, err := r.Verifier.Verify(blob, layer.BlobSum)
	blob.Close()
	if err != nil {
		return 0, err
	}

	log.Debug("verified")
	defer verifiedBlob.Close()

	log.Debug("registering")
	err = r.Cake.RegisterWithQuota(&image.Image{
		ID:     hex(layer.StrongID),
		Parent: hex(layer.ParentStrongID),
		Size:   size,
	}, verifiedBlob, quota)
	if err != nil {
		if strings.Contains(err.Error(), "unexpected EOF") {
			err = fmt.Errorf("%v. Possible cause: %v", err, quotaedreader.NewQuotaExceededErr())
		}
		return 0, err
	}

	return size, nil
}

func applyQuota(r io.ReadCloser, quota int64) io.ReadCloser {
	if quota < 0 {
		return r
	}
	return quotaedreader.New(r, quota)
}

//go:generate counterfeiter . Dialer
type Dialer interface {
	Dial(logger lager.Logger, host, repo, username, password string) (distclient.Conn, error)
}

func keys(m map[string]struct{}) (r []string) {
	for k, _ := range m {
		r = append(r, k)
	}
	return
}

func hex(d digest.Digest) string {
	if d == "" {
		return ""
	}

	return d.Hex()
}
