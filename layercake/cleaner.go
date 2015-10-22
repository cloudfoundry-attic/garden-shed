package layercake

import (
	"sync"

	"github.com/docker/docker/image"

	"github.com/pivotal-golang/lager"
)

type OvenCleaner struct {
	Cake

	Logger lager.Logger

	EnableImageCleanup bool

	retainedImages   map[string]struct{}
	retainedImagesMu sync.RWMutex
}

func (g *OvenCleaner) Get(id ID) (*image.Image, error) {
	return g.Cake.Get(id)
}

func (g *OvenCleaner) Remove(id ID) error {
	log := g.Logger.Session("remove", lager.Data{"ID": id})
	log.Info("start")

	if g.isRetained(id) {
		log.Info("layer-is-held")
		return nil
	}

	img, err := g.Cake.Get(id)
	if err != nil {
		log.Error("get-image", err)
		return nil
	}

	if err := g.Cake.Remove(id); err != nil {
		return err
	}

	if !g.EnableImageCleanup {
		return nil
	}

	if img.Parent == "" {
		return nil
	}
	if leaf, err := g.Cake.IsLeaf(DockerImageID(img.Parent)); err == nil && leaf {
		return g.Remove(DockerImageID(img.Parent))
	}

	return nil
}

func (g *OvenCleaner) Retain(id ID) {
	g.retainedImagesMu.Lock()
	defer g.retainedImagesMu.Unlock()

	if g.retainedImages == nil {
		g.retainedImages = make(map[string]struct{})
	}

	g.retainedImages[id.GraphID()] = struct{}{}
}

func (g *OvenCleaner) isRetained(id ID) bool {
	g.retainedImagesMu.Lock()
	defer g.retainedImagesMu.Unlock()

	if g.retainedImages == nil {
		return false
	}

	_, ok := g.retainedImages[id.GraphID()]
	return ok
}
