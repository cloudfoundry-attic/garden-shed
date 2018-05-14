package idmapper_test

import (
	. "code.cloudfoundry.org/idmapper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Idmapping", func() {
	Describe("Map", func() {
		Context("when the mapping does not contain the given id", func() {
			It("returns the original id", func() {
				mapping := MappingList{}
				Expect(mapping.Map(55)).To(BeEquivalentTo(55))
			})
		})

		Context("when the mapping contains the given id but the range size is zero", func() {
			It("returns the original id", func() {
				mapping := MappingList{{
					ContainerID: 55,
					HostID:      77,
					Size:        0,
				}}

				Expect(mapping.Map(55)).To(BeEquivalentTo(55))
			})
		})

		Context("when the mapping contains the given id as the first element of a range", func() {
			It("returns the mapped id", func() {
				mapping := MappingList{{
					ContainerID: 55,
					HostID:      77,
					Size:        1,
				}}

				Expect(mapping.Map(55)).To(BeEquivalentTo(77))
			})
		})

		Context("when the mapping contains the given id as path of a range", func() {
			It("returns the mapped id", func() {
				mapping := MappingList{{
					ContainerID: 55,
					HostID:      77,
					Size:        10,
				}}

				Expect(mapping.Map(64)).To(BeEquivalentTo(86))
			})
		})

		Context("when the uid is just outside of the range of a mapping (defensive)", func() {
			It("returns the original id", func() {
				mapping := MappingList{{
					ContainerID: 55,
					HostID:      77,
					Size:        10,
				}}

				Expect(mapping.Map(65)).To(BeEquivalentTo(65))

			})
		})
	})

	Describe("String", func() {
		Context("when the mapping is empty", func() {
			It("returns the string 'empty'", func() {
				mapping := MappingList{}
				Expect(mapping.String()).To(Equal("empty"))
			})
		})

		Context("when the mapping has a single entry", func() {
			It("returns a valid representation", func() {
				mapping := MappingList{
					{
						ContainerID: 122,
						HostID:      123456,
						Size:        125000,
					},
				}

				Expect(mapping.String()).To(Equal("122-123456-125000"))
			})
		})

		Context("when the mapping has multiple entries", func() {
			It("returns a valid representation containing all the entries", func() {
				mapping := MappingList{
					{
						ContainerID: 1,
						HostID:      2,
						Size:        3,
					},
					{
						ContainerID: 4,
						HostID:      5,
						Size:        6,
					},
				}

				Expect(mapping.String()).To(Equal("1-2-3,4-5-6"))
			})
		})
	})
})
