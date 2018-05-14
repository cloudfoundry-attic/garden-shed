package bundlerules

import (
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"github.com/mitchellh/copystructure"
)

type Base struct {
	PrivilegedBase   goci.Bndl
	UnprivilegedBase goci.Bndl
}

func (r Base) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec, _ string) (goci.Bndl, error) {
	if spec.Privileged {
		copiedBndl, err := copystructure.Copy(r.PrivilegedBase)
		if err != nil {
			return goci.Bndl{}, err
		}
		return copiedBndl.(goci.Bndl), nil
	} else {
		copiedBndl, err := copystructure.Copy(r.UnprivilegedBase)
		if err != nil {
			return goci.Bndl{}, err
		}
		return copiedBndl.(goci.Bndl), nil
	}
}
