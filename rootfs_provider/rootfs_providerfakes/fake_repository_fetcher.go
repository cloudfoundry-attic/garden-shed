// This file was generated by counterfeiter
package rootfs_providerfakes

import (
	"net/url"
	"sync"

	"code.cloudfoundry.org/garden-shed/repository_fetcher"
	"code.cloudfoundry.org/garden-shed/rootfs_provider"
)

type FakeRepositoryFetcher struct {
	FetchStub        func(*url.URL, int64) (*repository_fetcher.Image, error)
	fetchMutex       sync.RWMutex
	fetchArgsForCall []struct {
		arg1 *url.URL
		arg2 int64
	}
	fetchReturns struct {
		result1 *repository_fetcher.Image
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeRepositoryFetcher) Fetch(arg1 *url.URL, arg2 int64) (*repository_fetcher.Image, error) {
	fake.fetchMutex.Lock()
	fake.fetchArgsForCall = append(fake.fetchArgsForCall, struct {
		arg1 *url.URL
		arg2 int64
	}{arg1, arg2})
	fake.recordInvocation("Fetch", []interface{}{arg1, arg2})
	fake.fetchMutex.Unlock()
	if fake.FetchStub != nil {
		return fake.FetchStub(arg1, arg2)
	} else {
		return fake.fetchReturns.result1, fake.fetchReturns.result2
	}
}

func (fake *FakeRepositoryFetcher) FetchCallCount() int {
	fake.fetchMutex.RLock()
	defer fake.fetchMutex.RUnlock()
	return len(fake.fetchArgsForCall)
}

func (fake *FakeRepositoryFetcher) FetchArgsForCall(i int) (*url.URL, int64) {
	fake.fetchMutex.RLock()
	defer fake.fetchMutex.RUnlock()
	return fake.fetchArgsForCall[i].arg1, fake.fetchArgsForCall[i].arg2
}

func (fake *FakeRepositoryFetcher) FetchReturns(result1 *repository_fetcher.Image, result2 error) {
	fake.FetchStub = nil
	fake.fetchReturns = struct {
		result1 *repository_fetcher.Image
		result2 error
	}{result1, result2}
}

func (fake *FakeRepositoryFetcher) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.fetchMutex.RLock()
	defer fake.fetchMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeRepositoryFetcher) recordInvocation(key string, args []interface{}) {
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

var _ rootfs_provider.RepositoryFetcher = new(FakeRepositoryFetcher)