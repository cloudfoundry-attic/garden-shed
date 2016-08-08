// This file was generated by counterfeiter
package quota_managerfakes

import (
	"sync"

	"code.cloudfoundry.org/garden-shed/quota_manager"
)

type FakeAUFSDiffPathFinder struct {
	GetDiffLayerPathStub        func(rootFSPath string) string
	getDiffLayerPathMutex       sync.RWMutex
	getDiffLayerPathArgsForCall []struct {
		rootFSPath string
	}
	getDiffLayerPathReturns struct {
		result1 string
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeAUFSDiffPathFinder) GetDiffLayerPath(rootFSPath string) string {
	fake.getDiffLayerPathMutex.Lock()
	fake.getDiffLayerPathArgsForCall = append(fake.getDiffLayerPathArgsForCall, struct {
		rootFSPath string
	}{rootFSPath})
	fake.recordInvocation("GetDiffLayerPath", []interface{}{rootFSPath})
	fake.getDiffLayerPathMutex.Unlock()
	if fake.GetDiffLayerPathStub != nil {
		return fake.GetDiffLayerPathStub(rootFSPath)
	} else {
		return fake.getDiffLayerPathReturns.result1
	}
}

func (fake *FakeAUFSDiffPathFinder) GetDiffLayerPathCallCount() int {
	fake.getDiffLayerPathMutex.RLock()
	defer fake.getDiffLayerPathMutex.RUnlock()
	return len(fake.getDiffLayerPathArgsForCall)
}

func (fake *FakeAUFSDiffPathFinder) GetDiffLayerPathArgsForCall(i int) string {
	fake.getDiffLayerPathMutex.RLock()
	defer fake.getDiffLayerPathMutex.RUnlock()
	return fake.getDiffLayerPathArgsForCall[i].rootFSPath
}

func (fake *FakeAUFSDiffPathFinder) GetDiffLayerPathReturns(result1 string) {
	fake.GetDiffLayerPathStub = nil
	fake.getDiffLayerPathReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeAUFSDiffPathFinder) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getDiffLayerPathMutex.RLock()
	defer fake.getDiffLayerPathMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeAUFSDiffPathFinder) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ quota_manager.AUFSDiffPathFinder = new(FakeAUFSDiffPathFinder)