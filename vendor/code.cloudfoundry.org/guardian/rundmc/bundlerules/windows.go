package bundlerules

import (
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type Windows struct{}

func (w Windows) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec, _ string) (goci.Bndl, error) {
	if spec.BaseConfig.Windows == nil {
		return bndl, nil
	}

	bndl = bndl.WithWindows(*spec.BaseConfig.Windows)
	limit := uint64(spec.Limits.Memory.LimitInBytes)
	bndl = bndl.WithWindowsMemoryLimit(specs.WindowsMemoryResources{Limit: &limit})
	shares := uint16(spec.Limits.CPU.LimitInShares)
	bndl = bndl.WithWindowsCPUShares(specs.WindowsCPUResources{Shares: &shares})
	return bndl, nil
}
