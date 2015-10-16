package distclient

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/docker/docker/image"

	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/pivotal-golang/lager"
	"golang.org/x/net/context"
)

//go:generate counterfeiter -o fake_distclient/fake_conn.go . Conn
type Conn interface {
	GetManifest(logger lager.Logger, tag string) (*Manifest, error)
	GetBlobReader(logger lager.Logger, d digest.Digest) (io.Reader, error)
}

type conn struct {
	client distribution.Repository
}

type Manifest struct {
	Layers []image.Image
}

func Dial(logger lager.Logger, host, repo string) (Conn, error) {
	transport, err := NewTransport(logger, host, repo)
	if err != nil {
		logger.Error("failed-to-construct-transport", err)
		return nil, err
	}

	repoClient, err := client.NewRepository(context.TODO(), repo, host, transport)
	if err != nil {
		logger.Error("failed-to-construct-repository", err)
		return nil, err
	}

	return &conn{client: repoClient}, nil
}

func (r *conn) GetManifest(logger lager.Logger, tag string) (*Manifest, error) {
	manifestService, err := r.client.Manifests(context.TODO())
	if err != nil {
		logger.Error("failed-to-construct-manifest-service", err)
		return nil, err
	}

	layer, err := manifestService.GetByTag(tag)
	if err != nil {
		logger.Error("failed-to-get-by-tag", err)
		return nil, err
	}

	layers, err := toLayers(layer.FSLayers, layer.History)
	if err != nil {
		logger.Error("failed-to-get-v1-compat-layers", err)
		return nil, err
	}

	return &Manifest{Layers: layers}, nil
}

func (r *conn) GetBlobReader(logger lager.Logger, digest digest.Digest) (io.Reader, error) {
	blobStore := r.client.Blobs(context.TODO())
	return blobStore.Open(context.TODO(), digest)
}

func toLayers(fsl []manifest.FSLayer, history []manifest.History) (r []image.Image, err error) {
	for i := len(fsl) - 1; i >= 0; i-- {
		var image image.Image
		err := json.Unmarshal([]byte(history[i].V1Compatibility), &image)
		if err != nil {
			return nil, err
		}

		image.LayerID = fsl[i].BlobSum
		r = append(r, image)
	}

	return
}

func NewTransport(logger lager.Logger, host, repo string) (http.RoundTripper, error) {
	baseTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).Dial,
		DisableKeepAlives: true,
	}

	authTransport := transport.NewTransport(baseTransport)

	pingClient := &http.Client{
		Transport: authTransport,
		Timeout:   5 * time.Second,
	}

	req, err := http.NewRequest("GET", host+"/v2", nil)
	if err != nil {
		logger.Error("failed-to-create-ping-request", err)
		return nil, err
	}

	challengeManager := auth.NewSimpleChallengeManager()

	resp, err := pingClient.Do(req)
	if err != nil {
		logger.Error("failed-to-ping-registry", err)
		return nil, err
	} else {
		defer resp.Body.Close()

		if err := challengeManager.AddResponse(resp); err != nil {
			logger.Error("failed-to-add-response-to-challenge-manager", err)
			return nil, err
		}
	}

	credentialStore := dumbCredentialStore{"", ""}
	tokenHandler := auth.NewTokenHandler(authTransport, credentialStore, repo, "pull")
	basicHandler := auth.NewBasicHandler(credentialStore)
	authorizer := auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler)

	return transport.NewTransport(baseTransport, authorizer), nil
}

type dumbCredentialStore struct {
	username string
	password string
}

func (dcs dumbCredentialStore) Basic(*url.URL) (string, string) {
	return dcs.username, dcs.password
}
