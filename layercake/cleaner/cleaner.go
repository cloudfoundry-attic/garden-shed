package cleaner

import (
	"sync"

	"github.com/cloudfoundry-incubator/garden-shed/layercake"
	"github.com/pivotal-golang/lager"
)

type OvenCleaner struct {
	GraphCleanupThreshold Threshold
	retainCheck           Checker
}

type Checker interface {
	Check(id layercake.ID) bool
}

type RetainChecker interface {
	layercake.Retainer
	Checker
}

//go:generate counterfeiter . Threshold
type Threshold interface {
	Exceeded(log lager.Logger, cake layercake.Cake) bool
}

func NewOvenCleaner(retainCheck Checker, graphCleanupThreshold Threshold) *OvenCleaner {
	return &OvenCleaner{
		GraphCleanupThreshold: graphCleanupThreshold,
		retainCheck:           retainCheck,
	}
}

func (g *OvenCleaner) GC(log lager.Logger, cake layercake.Cake) error {
	log = log.Session("gc")
	log.Info("start")

	if exceeded := g.GraphCleanupThreshold.Exceeded(log, cake); !exceeded {
		log.Debug("threshold-not-exceeded")
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

func (g *OvenCleaner) removeRecursively(log lager.Logger, cake layercake.Cake, id layercake.ID) error {
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

	if leaf, err := cake.IsLeaf(layercake.DockerImageID(img.Parent)); err == nil && leaf {
		log.Debug("has-parent-leaf", lager.Data{"parent-id": img.Parent})
		return g.removeRecursively(log, cake, layercake.DockerImageID(img.Parent))
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

func (g *retainer) Retain(log lager.Logger, id layercake.ID) {
	g.retainedImagesMu.Lock()
	defer g.retainedImagesMu.Unlock()

	if g.retainedImages == nil {
		g.retainedImages = make(map[string]struct{})
	}

	g.retainedImages[id.GraphID()] = struct{}{}
}

func (g *retainer) Check(id layercake.ID) bool {
	g.retainedImagesMu.Lock()
	defer g.retainedImagesMu.Unlock()

	if g.retainedImages == nil {
		return false
	}

	_, ok := g.retainedImages[id.GraphID()]
	return ok
}

type CheckFunc func(id layercake.ID) bool

func (fn CheckFunc) Check(id layercake.ID) bool { return fn(id) }
