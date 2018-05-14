package bundlerules_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runtime-spec/specs-go"

	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

var _ = Describe("Base", func() {
	var (
		privilegeBndl, unprivilegeBndl goci.Bndl

		rule bundlerules.Base
	)

	BeforeEach(func() {
		t := true
		privilegeBndl = goci.Bndl{
			Spec: specs.Spec{
				Linux: &specs.Linux{
					Resources: &specs.LinuxResources{
						Memory: &specs.LinuxMemory{
							DisableOOMKiller: &t,
						},
					},
				},
			},
		}.WithNamespace(goci.NetworkNamespace)
		unprivilegeBndl = goci.Bndl{
			Spec: specs.Spec{
				Linux: &specs.Linux{
					Resources: &specs.LinuxResources{
						Memory: &specs.LinuxMemory{
							DisableOOMKiller: &t,
						},
					},
				},
			},
		}.WithNamespace(goci.UserNamespace)

		rule = bundlerules.Base{
			PrivilegedBase:   privilegeBndl,
			UnprivilegedBase: unprivilegeBndl,
		}
	})

	Context("when it is privileged", func() {
		It("should use the correct base", func() {
			retBndl, err := rule.Apply(goci.Bndl{}, spec.DesiredContainerSpec{
				Privileged: true,
			}, "not-needed-path")
			Expect(err).NotTo(HaveOccurred())
			Expect(retBndl).To(Equal(privilegeBndl))
		})

		It("returns a copy of the original Bndl data structure", func() {
			retBndl, err := rule.Apply(goci.Bndl{}, spec.DesiredContainerSpec{
				Privileged: true,
			}, "not-needed-path")
			Expect(err).NotTo(HaveOccurred())

			// Spec.Linux.Resources is a pointer
			Expect(retBndl.Spec.Linux.Resources.Memory.DisableOOMKiller).NotTo(BeIdenticalTo(privilegeBndl.Spec.Linux.Resources.Memory.DisableOOMKiller))
		})
	})

	Context("when it is not privileged", func() {
		It("should use the correct base", func() {
			retBndl, err := rule.Apply(goci.Bndl{}, spec.DesiredContainerSpec{
				Privileged: false,
			}, "not-needed-path")
			Expect(err).NotTo(HaveOccurred())

			Expect(retBndl).To(Equal(unprivilegeBndl))
		})

		It("returns a copy of the original Bndl data structure", func() {
			retBndl, err := rule.Apply(goci.Bndl{}, spec.DesiredContainerSpec{
				Privileged: false,
			}, "not-needed-path")
			Expect(err).NotTo(HaveOccurred())

			// Spec.Linux.Resources is a pointer
			Expect(retBndl.Spec.Linux.Resources.Memory.DisableOOMKiller).NotTo(BeIdenticalTo(unprivilegeBndl.Spec.Linux.Resources.Memory.DisableOOMKiller))
		})
	})
})
