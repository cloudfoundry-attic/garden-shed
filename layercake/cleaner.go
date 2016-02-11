package layercake

import (
	"sync"

	"github.com/pivotal-golang/lager"
)

type OvenCleaner struct {
	EnableImageCleanup bool
	retainCheck        Checker
}

type Checker interface {
	Check(id ID) bool
}

type RetainChecker interface {
	Retainer
	Checker
}

func NewOvenCleaner(retainCheck Checker, enableCleanup bool) *OvenCleaner {
	return &OvenCleaner{
		EnableImageCleanup: enableCleanup,
		retainCheck:        retainCheck,
	}
}

func (g *OvenCleaner) GC(log lager.Logger, cake Cake) error {
	log = log.Session("gc")
	log.Info("start")

	if !g.EnableImageCleanup {
		log.Debug("stop-image-cleanup-disabled")
		return nil
	}

	ids, err := cake.GetAllLeaves()
	if err != nil {
		return err
	}

	for _, id := range ids {
		if err := g.removeRecursively(log, cake, id); err != nil {
			return err
		}
	}

	return nil
}

func (g *OvenCleaner) removeRecursively(log lager.Logger, cake Cake, id ID) error {
	if g.retainCheck.Check(id) {
		log.Info("layer-is-held")
		return nil
	}

	img, err := cake.Get(id)
	if err != nil {
		log.Error("get-image", err)
		return nil
	}

	if img.Container != "" {
		log.Debug("image-is-container", lager.Data{"id": id, "container": img.Container})
		return nil
	}

	if err := cake.Remove(id); err != nil {
		log.Error("remove-image", err)
		return err
	}

	if img.Parent == "" {
		log.Debug("stop-image-has-no-parent")
		return nil
	}

	if leaf, err := cake.IsLeaf(DockerImageID(img.Parent)); err == nil && leaf {
		log.Debug("has-parent-leaf", lager.Data{"parent-id": img.Parent})
		return g.removeRecursively(log, cake, DockerImageID(img.Parent))
	}

	log.Info("finish")

	return nil
}

type retainer struct {
	retainedImages   map[string]struct{}
	retainedImagesMu sync.RWMutex
}

func NewRetainer() *retainer {
	return &retainer{}
}

func (g *retainer) Retain(log lager.Logger, id ID) {
	g.retainedImagesMu.Lock()
	defer g.retainedImagesMu.Unlock()

	if g.retainedImages == nil {
		g.retainedImages = make(map[string]struct{})
	}

	g.retainedImages[id.GraphID()] = struct{}{}
}

func (g *retainer) Check(id ID) bool {
	g.retainedImagesMu.Lock()
	defer g.retainedImagesMu.Unlock()

	if g.retainedImages == nil {
		return false
	}

	_, ok := g.retainedImages[id.GraphID()]
	return ok
}

type CheckFunc func(id ID) bool

func (fn CheckFunc) Check(id ID) bool { return fn(id) }
