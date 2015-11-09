package layercake_test

import (
	"errors"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"os"

	"github.com/cloudfoundry-incubator/garden-shed/layercake"
	"github.com/cloudfoundry-incubator/garden-shed/layercake/fake_cake"
	"github.com/cloudfoundry-incubator/garden-shed/layercake/fake_id"
	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
	"github.com/docker/docker/image"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Aufs", func() {
	var (
		aufsCake          *layercake.AufsCake
		cake              *fake_cake.FakeCake
		parentID          *fake_id.FakeID
		childID           *fake_id.FakeID
		testError         error
		namespacedChildID layercake.ID
		runner            command_runner.CommandRunner
	)

	BeforeEach(func() {
		cake = new(fake_cake.FakeCake)
		runner = linux_command_runner.New()

		parentID = new(fake_id.FakeID)
		parentID.GraphIDReturns("graph-id")

		childID = new(fake_id.FakeID)
		testError = errors.New("bad")
		namespacedChildID = layercake.NamespacedID(parentID, "test")
	})

	JustBeforeEach(func() {
		aufsCake = &layercake.AufsCake{
			Cake:   cake,
			Runner: runner,
		}
	})

	Describe("DriverName", func() {
		BeforeEach(func() {
			cake.DriverNameReturns("driver-name")
		})
		It("should delegate to the cake", func() {
			dn := aufsCake.DriverName()
			Expect(cake.DriverNameCallCount()).To(Equal(1))
			Expect(dn).To(Equal("driver-name"))
		})
	})

	Describe("Create", func() {
		Context("when the child ID is namespaced", func() {
			It("should delegate to the cake but with an empty parent", func() {
				cake.CreateReturns(testError)
				Expect(aufsCake.Create(namespacedChildID, parentID)).To(Equal(testError))
				Expect(cake.CreateCallCount()).To(Equal(1))
				cid, iid := cake.CreateArgsForCall(0)
				Expect(cid).To(Equal(namespacedChildID))
				Expect(iid.GraphID()).To(BeEmpty())
			})

			Context("when mounting child fails", func() {
				It("should return the error", func() {
					cake.GetReturns(nil, testError)
					Expect(aufsCake.Create(namespacedChildID, parentID)).To(Equal(testError))
				})
			})

			Context("when getting parent's path fails", func() {
				It("should return the error", func() {
					cake.PathReturns("", testError)
					Expect(aufsCake.Create(namespacedChildID, parentID)).To(Equal(testError))
				})
			})

			Context("when getting child's path fails", func() {
				It("should return the error", func() {
					cake.PathStub = func(id layercake.ID) (string, error) {
						if id == parentID {
							return "/path/to/the/parent", nil
						}

						if id == namespacedChildID {
							return "", testError
						}

						return "", nil
					}

					Expect(aufsCake.Create(namespacedChildID, parentID)).To(Equal(testError))
				})
			})

			Describe("Copying", func() {
				var (
					parentDir          string
					namespacedChildDir string
				)

				BeforeEach(func() {
					var err error
					parentDir, err = ioutil.TempDir("", "parent-layer")
					Expect(err).NotTo(HaveOccurred())

					namespacedChildDir, err = ioutil.TempDir("", "child-layer")
					Expect(err).NotTo(HaveOccurred())

					cake.PathStub = func(id layercake.ID) (string, error) {
						if id == parentID {
							return parentDir, nil
						}

						if id == namespacedChildID {
							return namespacedChildDir, nil
						}
						return "", nil
					}
				})

				Context("when parent layer has a file", func() {
					BeforeEach(func() {
						Expect(ioutil.WriteFile(filepath.Join(parentDir, "somefile"), []byte("somecontents"), 0755)).To(Succeed())
					})

					It("should copy the parent layer to the child layer", func() {
						Expect(aufsCake.Create(namespacedChildID, parentID)).To(Succeed())

						Expect(cake.CreateCallCount()).To(Equal(1))
						layerID, layerParentID := cake.CreateArgsForCall(0)
						Expect(layerID).To(Equal(namespacedChildID))
						Expect(layerParentID).To(Equal(layercake.DockerImageID("")))

						Expect(cake.GetCallCount()).To(Equal(1))
						Expect(cake.GetArgsForCall(0)).To(Equal(namespacedChildID))

						Expect(cake.PathCallCount()).To(Equal(2))
						Expect(cake.PathArgsForCall(0)).To(Equal(parentID))
						Expect(cake.PathArgsForCall(1)).To(Equal(namespacedChildID))

						_, err := os.Stat(filepath.Join(namespacedChildDir, "somefile"))
						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("when parent layer has a directory", func() {
					var subDirectory string

					BeforeEach(func() {
						subDirectory = filepath.Join(parentDir, "sub-dir")
						Expect(os.MkdirAll(subDirectory, 0755)).To(Succeed())
						Expect(ioutil.WriteFile(filepath.Join(subDirectory, ".some-hidden-file"), []byte("somecontents"), 0755)).To(Succeed())
					})

					It("should copy the parent layer to the child layer", func() {
						Expect(aufsCake.Create(namespacedChildID, parentID)).To(Succeed())

						Expect(cake.CreateCallCount()).To(Equal(1))
						layerID, layerParentID := cake.CreateArgsForCall(0)
						Expect(layerID).To(Equal(namespacedChildID))
						Expect(layerParentID).To(Equal(layercake.DockerImageID("")))

						Expect(cake.GetCallCount()).To(Equal(1))
						Expect(cake.GetArgsForCall(0)).To(Equal(namespacedChildID))

						Expect(cake.PathCallCount()).To(Equal(2))
						Expect(cake.PathArgsForCall(0)).To(Equal(parentID))
						Expect(cake.PathArgsForCall(1)).To(Equal(namespacedChildID))

						_, err := os.Stat(filepath.Join(subDirectory, ".some-hidden-file"))
						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("when parent layer has a hidden file", func() {
					BeforeEach(func() {
						Expect(ioutil.WriteFile(filepath.Join(parentDir, ".some-hidden-file"), []byte("somecontents"), 0755)).To(Succeed())
					})

					It("should copy the parent layer to the child layer", func() {
						Expect(aufsCake.Create(namespacedChildID, parentID)).To(Succeed())

						Expect(cake.CreateCallCount()).To(Equal(1))
						layerID, layerParentID := cake.CreateArgsForCall(0)
						Expect(layerID).To(Equal(namespacedChildID))
						Expect(layerParentID).To(Equal(layercake.DockerImageID("")))

						Expect(cake.GetCallCount()).To(Equal(1))
						Expect(cake.GetArgsForCall(0)).To(Equal(namespacedChildID))

						Expect(cake.PathCallCount()).To(Equal(2))
						Expect(cake.PathArgsForCall(0)).To(Equal(parentID))
						Expect(cake.PathArgsForCall(1)).To(Equal(namespacedChildID))

						_, err := os.Stat(filepath.Join(namespacedChildDir, ".some-hidden-file"))
						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("when command runner fails", func() {
					testError := errors.New("oh no!")
					BeforeEach(func() {
						fakeRunner := fake_command_runner.New()
						fakeRunner.WhenRunning(fake_command_runner.CommandSpec{}, func(cmd *exec.Cmd) error {
							return testError
						})

						runner = fakeRunner
					})

					It("returns the error", func() {
						Expect(aufsCake.Create(namespacedChildID, parentID)).To(Equal(testError))
					})
				})
			})
		})

		Context("when the image ID is not namespaced", func() {
			It("should delegate to the cake", func() {
				cake.CreateReturns(testError)
				Expect(aufsCake.Create(childID, parentID)).To(Equal(testError))
				Expect(cake.CreateCallCount()).To(Equal(1))
				cid, iid := cake.CreateArgsForCall(0)
				Expect(cid).To(Equal(childID))
				Expect(iid).To(Equal(parentID))
			})
		})

	})

	Describe("Get", func() {
		var testImage *image.Image

		BeforeEach(func() {
			testImage = &image.Image{}
			cake.GetReturns(testImage, testError)
		})

		It("should delegate to the cake", func() {
			img, err := aufsCake.Get(childID)
			Expect(img).To(Equal(testImage))
			Expect(err).To(Equal(testError))
		})
	})

	Describe("Remove", func() {
		BeforeEach(func() {
			cake.RemoveReturns(testError)
		})

		It("should delegate to the cake", func() {
			Expect(aufsCake.Remove(childID)).To(Equal(testError))
		})
	})

	Describe("IsLeaf", func() {
		BeforeEach(func() {
			cake.IsLeafReturns(true, testError)
		})

		It("should delegate to the cake", func() {
			isLeaf, err := aufsCake.IsLeaf(childID)
			Expect(isLeaf).To(BeTrue())
			Expect(err).To(Equal(testError))
		})
	})
})
