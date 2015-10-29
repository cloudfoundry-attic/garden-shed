package aufs_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/garden-shed/docker_drivers/aufs"
	"github.com/cloudfoundry-incubator/garden-shed/docker_drivers/aufs/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("QuotaedDriver", func() {
	var (
		fakeGraphDriver     *fakes.FakeGraphDriver
		fakeLoopMounter     *fakes.FakeLoopMounter
		fakeBackingStoreMgr *fakes.FakeBackingStoreMgr

		driver *aufs.QuotaedDriver

		rootPath string
	)

	BeforeEach(func() {
		fakeGraphDriver = new(fakes.FakeGraphDriver)
		fakeLoopMounter = new(fakes.FakeLoopMounter)
		fakeBackingStoreMgr = new(fakes.FakeBackingStoreMgr)

		rootPath = "/path/to/my/banana/graph"
		driver = &aufs.QuotaedDriver{
			GraphDriver:     fakeGraphDriver,
			BackingStoreMgr: fakeBackingStoreMgr,
			LoopMounter:     fakeLoopMounter,
			RootPath:        rootPath,
		}
	})

	Describe("GetQuotaed", func() {
		It("should create a backing store file", func() {
			id := "banana-id"
			quota := int64(12 * 1024)

			_, err := driver.GetQuotaed(id, "", quota)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBackingStoreMgr.CreateCallCount()).To(Equal(1))
			gottenId, gottenQuota := fakeBackingStoreMgr.CreateArgsForCall(0)
			Expect(gottenId).To(Equal(id))
			Expect(gottenQuota).To(Equal(quota))
		})

		Context("when failing to create a backing store", func() {
			It("should return an error", func() {
				fakeBackingStoreMgr.CreateReturns("", errors.New("create failed!"))

				_, err := driver.GetQuotaed("banana-id", "", 12*1024)
				Expect(err).To(MatchError("create failed!"))
			})
		})

		It("should mount the backing store file", func() {
			realDevicePath := "/path/to/my/banana/device"

			fakeBackingStoreMgr.CreateReturns(realDevicePath, nil)

			_, err := driver.GetQuotaed("banana-id", "", 10*1024)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLoopMounter.MountFileCallCount()).To(Equal(1))
			devicePath, destPath := fakeLoopMounter.MountFileArgsForCall(0)
			Expect(devicePath).To(Equal(realDevicePath))
			Expect(destPath).To(Equal("/path/to/my/banana/graph/aufs/diff/banana-id"))
		})

		Context("when failing to mount the backing store", func() {
			BeforeEach(func() {
				fakeLoopMounter.MountFileReturns(errors.New("another banana error"))
			})

			It("should return an error", func() {
				_, err := driver.GetQuotaed("banana-id", "", 10*1024)
				Expect(err).To(MatchError("another banana error"))
			})

			It("should not mount the layer", func() {
				driver.GetQuotaed("banana-id", "", 10*1024*1024)
				Expect(fakeGraphDriver.GetCallCount()).To(Equal(0))
			})
		})

		It("should mount the layer", func() {
			id := "mango-id"
			mountLabel := "wild mangos: handle with care"
			quota := int64(12 * 1024 * 1024)

			_, err := driver.GetQuotaed(id, mountLabel, quota)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeGraphDriver.GetCallCount()).To(Equal(1))
			gottenID, gottenMountLabel := fakeGraphDriver.GetArgsForCall(0)
			Expect(gottenID).To(Equal(id))
			Expect(gottenMountLabel).To(Equal(mountLabel))
		})

		It("should return the mounted layer's path", func() {
			mountPath := "/path/to/mounted/banana"

			fakeGraphDriver.GetReturns(mountPath, nil)

			path, err := driver.GetQuotaed("test-banana-id", "", 10*1024*1024)
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal(mountPath))
		})

		Context("when mounting the layer fails", func() {
			It("should return an error", func() {
				fakeGraphDriver.GetReturns("", errors.New("Another banana error"))

				_, err := driver.GetQuotaed("banana-id", "", 10*1024*1024)
				Expect(err).To(MatchError(ContainSubstring("Another banana error")))
			})
		})
	})

	Describe("Remove", func() {
		It("should put and remove the layer twice", func() {
			id := "herring-id"

			Expect(driver.Remove(id)).To(Succeed())

			Expect(fakeGraphDriver.PutCallCount()).To(Equal(2))
			Expect(fakeGraphDriver.PutArgsForCall(0)).To(Equal(id))
			Expect(fakeGraphDriver.RemoveCallCount()).To(Equal(2))
			Expect(fakeGraphDriver.RemoveArgsForCall(0)).To(Equal(id))
		})

		It("should unmount the loop mount", func() {
			Expect(driver.Remove("banana-id")).To(Succeed())

			Expect(fakeLoopMounter.UnmountCallCount()).To(Equal(1))
			Expect(fakeLoopMounter.UnmountArgsForCall(0)).To(Equal("/path/to/my/banana/graph/aufs/diff/banana-id"))
		})

		It("should delete the backing store", func() {
			id := "banana-id"

			driver.GetQuotaed(id, "", 10*1024)

			Expect(driver.Remove("banana-id")).To(Succeed())

			Expect(fakeBackingStoreMgr.DeleteCallCount()).To(Equal(1))
			Expect(fakeBackingStoreMgr.DeleteArgsForCall(0)).To(Equal(id))
		})
	})
})
