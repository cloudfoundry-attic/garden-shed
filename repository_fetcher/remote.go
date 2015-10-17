package repository_fetcher

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/docker/docker/image"

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
}

func NewRemote(logger lager.Logger, defaultHost string, cake layercake.Cake, dialer Dialer, verifier Verifier) *Remote {
	return &Remote{
		Logger:      logger,
		DefaultHost: defaultHost,
		Dial:        dialer,
		Cake:        cake,
		Verifier:    verifier,
	}
}

func (r *Remote) Fetch(u *url.URL, diskQuota int64) (*Image, error) {
	conn, manifest, err := r.manifest(u)
	if err != nil {
		return nil, err
	}

	var env []string
	var vols []string
	for _, layer := range manifest.Layers {
		if _, err := r.Cake.Get(layercake.DockerImageID(layer.ID)); err == nil {
			continue // got cache
		}

		if layer.Config != nil {
			env = append(env, layer.Config.Env...)
			vols = append(vols, keys(layer.Config.Volumes)...)
		}

		blob, err := conn.GetBlobReader(r.Logger, layer.LayerID)
		if err != nil {
			return nil, err
		}

		verifiedBlob, err := r.Verifier.Verify(blob, layer.LayerID)
		if err != nil {
			return nil, err
		}

		defer verifiedBlob.Close()
		if err := r.Cake.Register(&image.Image{ID: layer.ID, Parent: layer.Parent}, verifiedBlob); err != nil {
			return nil, err
		}
	}

	return &Image{
		ImageID: manifest.Layers[len(manifest.Layers)-1].ID,
		Env:     env,
		Volumes: vols,
	}, nil
}

func (r *Remote) FetchID(u *url.URL) (layercake.ID, error) {
	_, manifest, err := r.manifest(u)
	if err != nil {
		return nil, err
	}

	return layercake.DockerImageID(manifest.Layers[len(manifest.Layers)-1].ID), nil
}

func (r *Remote) manifest(u *url.URL) (distclient.Conn, *distclient.Manifest, error) {
	host := u.Host
	if host == "" {
		host = r.DefaultHost
	}

	path := u.Path[1:] // strip off initial '/'
	if strings.Index(path, "/") < 0 {
		path = "library/" + path
	}

	tag := u.Fragment
	if tag == "" {
		tag = "latest"
	}

	conn, err := r.Dial.Dial(r.Logger, "https://"+host, path)
	if err != nil {
		return nil, nil, err
	}

	manifest, err := conn.GetManifest(r.Logger, tag)
	if err != nil {
		return nil, nil, fmt.Errorf("get manifest for tag %s on repo %s: %s", u.Fragment, u, err)
	}

	return conn, manifest, err
}

//go:generate counterfeiter . Dialer
type Dialer interface {
	Dial(logger lager.Logger, host, repo string) (distclient.Conn, error)
}

type DialFunc func(logger lager.Logger, host, repo string) (distclient.Conn, error)

func (fn DialFunc) Dial(logger lager.Logger, host, repo string) (distclient.Conn, error) {
	return fn(logger, host, repo)
}

func keys(m map[string]struct{}) (r []string) {
	for k, _ := range m {
		r = append(r, k)
	}
	return
}
