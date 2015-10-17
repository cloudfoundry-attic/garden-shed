package repository_fetcher

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/docker/docker/image"

	"github.com/docker/distribution/digest"

	"github.com/cloudfoundry-incubator/garden-shed/distclient"
	"github.com/cloudfoundry-incubator/garden-shed/layercake"
	"github.com/pivotal-golang/lager"
)

type Remote struct {
	Logger      lager.Logger
	DefaultHost string
	Dial        Dialer
	Cake        layercake.Cake
	Verifier    Verifier

	FetchLock *FetchLock
}

func NewRemote(logger lager.Logger, defaultHost string, cake layercake.Cake, dialer Dialer, verifier Verifier) *Remote {
	return &Remote{
		Logger:      logger,
		DefaultHost: defaultHost,
		Dial:        dialer,
		Cake:        cake,
		Verifier:    verifier,
		FetchLock:   NewFetchLock(),
	}
}

func (r *Remote) Fetch(u *url.URL, diskQuota int64) (*Image, error) {
	log := r.Logger.Session("fetch", lager.Data{"url": u})

	log.Info("started")
	defer log.Info("complete")

	conn, manifest, err := r.manifest(log, u)
	if err != nil {
		return nil, err
	}

	var env []string
	var vols []string
	for _, layer := range manifest.Layers {
		if layer.Image.Config != nil {
			env = append(env, layer.Image.Config.Env...)
			vols = append(vols, keys(layer.Image.Config.Volumes)...)
		}

		if err := r.fetchLayer(log, conn, layer); err != nil {
			return nil, err
		}
	}

	return &Image{
		ImageID: hex(manifest.Layers[len(manifest.Layers)-1].StrongID),
		Env:     env,
		Volumes: vols,
	}, nil
}

func (r *Remote) FetchID(u *url.URL) (layercake.ID, error) {
	_, manifest, err := r.manifest(r.Logger.Session("fetch-id"), u)
	if err != nil {
		return nil, err
	}

	return layercake.DockerImageID(hex(manifest.Layers[len(manifest.Layers)-1].StrongID)), nil
}

func (r *Remote) manifest(log lager.Logger, u *url.URL) (distclient.Conn, *distclient.Manifest, error) {
	log = log.Session("get-manifest", lager.Data{"url": u})

	log.Info("started")
	defer log.Info("got")

	host := u.Host
	if host == "" {
		host = r.DefaultHost
	}

	path := u.Path[1:] // strip off initial '/'
	if host == r.DefaultHost && strings.Index(path, "/") < 0 {
		path = "library/" + path
	}

	tag := u.Fragment
	if tag == "" {
		tag = "latest"
	}

	conn, err := r.Dial.Dial(r.Logger, host, path)
	if err != nil {
		return nil, nil, err
	}

	manifest, err := conn.GetManifest(r.Logger, tag)
	if err != nil {
		return nil, nil, fmt.Errorf("get manifest for tag %s on repo %s: %s", u.Fragment, u, err)
	}

	return conn, manifest, err
}

func (r *Remote) fetchLayer(log lager.Logger, conn distclient.Conn, layer distclient.Layer) error {
	log = log.Session("fetch-layer", lager.Data{"blobsum": layer.BlobSum, "id": layer.StrongID, "parent": layer.ParentStrongID})

	log.Info("start")
	defer log.Info("fetched")

	r.FetchLock.Acquire(layer.BlobSum.String())
	defer r.FetchLock.Release(layer.BlobSum.String())

	_, err := r.Cake.Get(layercake.DockerImageID(hex(layer.StrongID)))
	if err == nil {
		log.Info("got-cache")
		return nil
	}

	blob, err := conn.GetBlobReader(r.Logger, layer.BlobSum)
	if err != nil {
		return err
	}

	log.Info("verifying")
	verifiedBlob, err := r.Verifier.Verify(blob, layer.BlobSum)
	if err != nil {
		return err
	}

	log.Info("verified")
	defer verifiedBlob.Close()

	log.Info("registering")
	err = r.Cake.Register(&image.Image{ID: hex(layer.StrongID), Parent: hex(layer.ParentStrongID)}, verifiedBlob)
	if err != nil {
		return err
	}

	return nil
}

//go:generate counterfeiter . Dialer
type Dialer interface {
	Dial(logger lager.Logger, host, repo string) (distclient.Conn, error)
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
