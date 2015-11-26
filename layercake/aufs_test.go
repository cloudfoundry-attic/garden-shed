package layercake_test

import (
	"errors"
	"fmt"
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
		aufsCake               *layercake.AufsCake
		cake                   *fake_cake.FakeCake
		parentID               *fake_id.FakeID
		childID                *fake_id.FakeID
		testError              error
		namespacedChildID      layercake.ID
		otherNamespacedChildID layercake.ID
		runner                 command_runner.CommandRunner
		baseDirectory          string
	)

	BeforeEach(func() {
		var err error
		baseDirectory, err = ioutil.TempDir("", "aufsTestGraphRoot")
		Expect(err).NotTo(HaveOccurred())

		cake = new(fake_cake.FakeCake)
		runner = linux_command_runner.New()

		parentID = new(fake_id.FakeID)
		parentID.GraphIDReturns("graph-id")

		childID = new(fake_id.FakeID)
		testError = errors.New("bad")

		namespacedChildID = layercake.NamespacedID(parentID, "test")
		otherNamespacedChildID = layercake.NamespacedID(parentID, "test2")
	})

	JustBeforeEach(func() {
		aufsCake = &layercake.AufsCake{
			Cake:      cake,
			Runner:    runner,
			GraphRoot: baseDirectory,
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
		var (
			parentDir               string
			namespacedChildDir      string
			otherNamespacedChildDir string
		)

		BeforeEach(func() {
			var err error
			parentDir, err = ioutil.TempDir("", "parent-layer")
			Expect(err).NotTo(HaveOccurred())

			namespacedChildDir, err = ioutil.TempDir("", "namespaced-child-layer")
			Expect(err).NotTo(HaveOccurred())

			otherNamespacedChildDir, err = ioutil.TempDir("", "other-namespaced-child-layer")
			Expect(err).NotTo(HaveOccurred())

			cake.PathStub = func(id layercake.ID) (string, error) {
				if id == parentID {
					return parentDir, nil
				}

				if id == namespacedChildID {
					return namespacedChildDir, nil
				}

				if id == otherNamespacedChildID {
					return otherNamespacedChildDir, nil
				}

				return "", nil
			}

			cake.IsLeafReturns(true, nil)
		})

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

					It("should not create the garden-info metadata directories", func() {
						Expect(aufsCake.Create(namespacedChildID, parentID)).To(Equal(testError))
						Expect(filepath.Join(baseDirectory, "garden-info")).NotTo(BeADirectory())
						Expect(filepath.Join(baseDirectory, "garden-info", "parent-child")).NotTo(BeADirectory())
						Expect(filepath.Join(baseDirectory, "garden-info", "child-parent")).NotTo(BeADirectory())
					})
				})
			})

			Describe("Parent-child relationship", func() {
				Context("when the namespaced layer is duplicated", func() {
					JustBeforeEach(func() {
						Expect(aufsCake.Create(namespacedChildID, parentID)).To(Succeed())
						Expect(aufsCake.Create(namespacedChildID, parentID)).To(MatchError(fmt.Sprintf("%s already exists", namespacedChildID.GraphID())))
					})

					It("does not add duplicated records in child-parent file", func() {
						childParentInfo := filepath.Join(baseDirectory, "garden-info", "child-parent", namespacedChildID.GraphID())
						Expect(aufsCake.Create(namespacedChildID, parentID)).To(HaveOccurred())

						childParentInfoData, err := ioutil.ReadFile(childParentInfo)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(childParentInfoData)).To(Equal(parentID.GraphID() + "\n"))
					})

					It("does not duplicate the namespaced child id in parent-child file", func() {
						parentChildInfo := filepath.Join(baseDirectory, "garden-info", "parent-child", parentID.GraphID())
						Expect(parentChildInfo).To(BeAnExistingFile())

						parentChildInfoData, err := ioutil.ReadFile(parentChildInfo)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(parentChildInfoData)).To(Equal(namespacedChildID.GraphID() + "\n"))
					})
				})

				Context("when creating the garden-info metadata directories fails", func() {
					JustBeforeEach(func() {
						aufsCake.GraphRoot = "\x00"
					})

					It("should return an error", func() {
						Expect(aufsCake.Create(namespacedChildID, parentID)).NotTo(Succeed())
					})
				})

				It("keeps parent-child relationship information", func() {
					cake.IsLeafReturns(true, nil)
					Expect(aufsCake.Create(namespacedChildID, parentID)).To(Succeed())

					isLeaf, err := aufsCake.IsLeaf(parentID)
					Expect(err).NotTo(HaveOccurred())
					Expect(isLeaf).To(BeFalse())
				})

				It("keeps child-parent relationship information", func() {
					cake.GetStub = func(id layercake.ID) (*image.Image, error) {
						if id != parentID &&
							id != childID &&
							id != namespacedChildID &&
							id != otherNamespacedChildID {
							return nil, testError
						}

						img := &image.Image{
							ID: id.GraphID(),
						}

						if id == childID {
							img.Parent = parentID.GraphID()
						}

						return img, nil
					}

					Expect(aufsCake.Create(namespacedChildID, parentID)).To(Succeed())
					img, err := aufsCake.Get(namespacedChildID)
					Expect(err).NotTo(HaveOccurred())
					Expect(img.Parent).To(Equal(parentID.GraphID()))
				})

				Context("when there are two namespaced children to one parent", func() {
					It("removing the first child doesn't destroy all the metadata", func() {
						Expect(aufsCake.Create(namespacedChildID, parentID)).To(Succeed())
						Expect(aufsCake.Create(otherNamespacedChildID, parentID)).To(Succeed())

						Expect(aufsCake.Remove(namespacedChildID)).To(Succeed())
						isLeaf, err := aufsCake.IsLeaf(parentID)
						Expect(err).NotTo(HaveOccurred())
						Expect(isLeaf).To(BeFalse())
					})

					It("removing the second child doesn't destroy all the metadata", func() {
						Expect(aufsCake.Create(namespacedChildID, parentID)).To(Succeed())
						Expect(aufsCake.Create(otherNamespacedChildID, parentID)).To(Succeed())

						Expect(aufsCake.Remove(otherNamespacedChildID)).To(Succeed())
						isLeaf, err := aufsCake.IsLeaf(parentID)
						Expect(err).NotTo(HaveOccurred())
						Expect(isLeaf).To(BeFalse())
					})

					It("keeps metadata on both of them", func() {
						Expect(aufsCake.Create(namespacedChildID, parentID)).To(Succeed())
						Expect(aufsCake.Create(otherNamespacedChildID, parentID)).To(Succeed())

						Expect(aufsCake.Remove(otherNamespacedChildID)).To(Succeed())
						Expect(aufsCake.Remove(namespacedChildID)).To(Succeed())
						isLeaf, err := aufsCake.IsLeaf(parentID)
						Expect(err).NotTo(HaveOccurred())
						Expect(isLeaf).To(BeTrue())
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
		Context("when the image ID is namespaced", func() {
			var (
				parentDir               string
				namespacedChildDir      string
				otherNamespacedChildDir string
			)

			JustBeforeEach(func() {
				var err error
				parentDir, err = ioutil.TempDir("", "parent-layer")
				Expect(err).NotTo(HaveOccurred())

				namespacedChildDir, err = ioutil.TempDir("", "namespaced-child-layer")
				Expect(err).NotTo(HaveOccurred())

				otherNamespacedChildDir, err = ioutil.TempDir("", "other-namespaced-child-layer")
				Expect(err).NotTo(HaveOccurred())

				cake.PathStub = func(id layercake.ID) (string, error) {
					if id == parentID {
						return parentDir, nil
					}

					if id == namespacedChildID {
						return namespacedChildDir, nil
					}

					if id == otherNamespacedChildID {
						return otherNamespacedChildDir, nil
					}

					return "", testError
				}

				Expect(aufsCake.Create(namespacedChildID, parentID)).To(Succeed())
				Expect(aufsCake.Create(otherNamespacedChildID, parentID)).To(Succeed())

				namespacedChildID = layercake.DockerImageID(namespacedChildID.GraphID())
				otherNamespacedChildID = layercake.DockerImageID(otherNamespacedChildID.GraphID())

				cake.GetStub = func(id layercake.ID) (*image.Image, error) {
					if id != parentID &&
						id != childID &&
						id != namespacedChildID &&
						id != otherNamespacedChildID {
						return nil, testError
					}

					img := &image.Image{
						ID: id.GraphID(),
					}

					if id == childID {
						img.Parent = parentID.GraphID()
					}

					return img, nil
				}
			})

			Context("when the image ID is an invalid file", func() {
				JustBeforeEach(func() {
					cake.GetReturns(&image.Image{}, nil)
				})

				It("returns the error", func() {
					childID.GraphIDReturns("\x00")

					img, err := aufsCake.Get(childID)
					Expect(img).To(BeNil())
					Expect(err).To(HaveOccurred())
				})
			})

			It("returns its parent", func() {
				img, err := aufsCake.Get(namespacedChildID)
				Expect(img).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				Expect(img.Parent).To(Equal(parentID.GraphID()))
			})
		})

		Context("when the image ID is not namespaced", func() {
			Context("when the underlying cake fails", func() {
				JustBeforeEach(func() {
					cake.GetReturns(nil, testError)
				})

				It("returns the error", func() {
					img, err := aufsCake.Get(childID)
					Expect(cake.GetCallCount()).To(Equal(1))
					Expect(cake.GetArgsForCall(0)).To(Equal(childID))
					Expect(img).To(BeNil())
					Expect(err).To(Equal(testError))
				})
			})

			Context("when the child-parent info does not exist", func() {
				It("should return the image a nil parent", func() {
					cake.GetReturns(&image.Image{}, nil)
					img, err := aufsCake.Get(childID)
					Expect(err).ToNot(HaveOccurred())
					Expect(img.Parent).To(BeEmpty())
				})
			})

			It("should delegate to the cake", func() {
				testImage := &image.Image{Parent: "this-parent"}
				cake.GetReturns(testImage, nil)

				img, err := aufsCake.Get(childID)
				Expect(cake.GetCallCount()).To(Equal(1))
				Expect(cake.GetArgsForCall(0)).To(Equal(childID))
				Expect(err).To(BeNil())
				Expect(img).To(Equal(testImage))
			})
		})
	})

	Describe("Remove", func() {
		Context("when the image ID is not namespaced", func() {
			It("should return the error when cake fails", func() {
				cake.RemoveReturns(testError)
				Expect(aufsCake.Remove(childID)).To(Equal(testError))
			})

			It("should delegate to the cake", func() {
				Expect(aufsCake.Remove(childID)).To(Succeed())
				Expect(cake.RemoveCallCount()).To(Equal(1))
				Expect(cake.RemoveArgsForCall(0)).To(Equal(childID))
			})
		})

		Context("when the image ID is namespaced", func() {
			var (
				parentDir               string
				namespacedChildDir      string
				otherNamespacedChildDir string
			)

			BeforeEach(func() {
				var err error
				parentDir, err = ioutil.TempDir("", "parent-layer")
				Expect(err).NotTo(HaveOccurred())

				namespacedChildDir, err = ioutil.TempDir("", "namespaced-child-layer")
				Expect(err).NotTo(HaveOccurred())

				otherNamespacedChildDir, err = ioutil.TempDir("", "other-namespaced-child-layer")
				Expect(err).NotTo(HaveOccurred())

				cake.IsLeafReturns(true, nil)

				cake.PathStub = func(id layercake.ID) (string, error) {
					if id == parentID {
						return parentDir, nil
					}

					if id == namespacedChildID {
						return namespacedChildDir, nil
					}

					if id == otherNamespacedChildID {
						return otherNamespacedChildDir, nil
					}

					return "", testError
				}
			})

			JustBeforeEach(func() {
				Expect(aufsCake.Create(namespacedChildID, parentID)).To(Succeed())
			})

			Context("when the base directory does not exist", func() {
				It("should silently succeed", func() {
					Expect(os.RemoveAll(baseDirectory)).To(Succeed())
					Expect(aufsCake.Remove(childID)).To(Succeed())
				})
			})

			Context("when it has sibling", func() {
				JustBeforeEach(func() {
					Expect(aufsCake.Create(otherNamespacedChildID, parentID)).To(Succeed())
				})

				It("should not make the parent a leaf when removing the first child", func() {
					Expect(aufsCake.Remove(namespacedChildID)).To(Succeed())
					isLeaf, err := aufsCake.IsLeaf(parentID)
					Expect(err).NotTo(HaveOccurred())
					Expect(isLeaf).To(BeFalse())
				})

				It("should not make the parent a leaf when removing the second child", func() {
					Expect(aufsCake.Remove(otherNamespacedChildID)).To(Succeed())
					isLeaf, err := aufsCake.IsLeaf(parentID)
					Expect(err).NotTo(HaveOccurred())
					Expect(isLeaf).To(BeFalse())
				})

				It("should not make the parent a leaf when removing the second child", func() {
					Expect(aufsCake.Remove(layercake.DockerImageID(otherNamespacedChildID.GraphID()))).To(Succeed())

					isLeaf, err := aufsCake.IsLeaf(parentID)
					Expect(err).NotTo(HaveOccurred())
					Expect(isLeaf).To(BeFalse())
				})

				It("should make the parent a leaf when removing both children", func() {
					Expect(aufsCake.Remove(layercake.DockerImageID(namespacedChildID.GraphID()))).To(Succeed())
					Expect(aufsCake.Remove(layercake.DockerImageID(otherNamespacedChildID.GraphID()))).To(Succeed())

					isLeaf, err := aufsCake.IsLeaf(parentID)
					Expect(err).NotTo(HaveOccurred())
					Expect(isLeaf).To(BeTrue())
				})
			})

			Context("when it does not have sibilings", func() {
				It("should make the parent a leaf", func() {
					Expect(aufsCake.Remove(namespacedChildID)).To(Succeed())

					isLeaf, err := aufsCake.IsLeaf(parentID)
					Expect(err).NotTo(HaveOccurred())
					Expect(isLeaf).To(BeTrue())
				})

				It("should remove the parent child relationship", func() {
					Expect(aufsCake.Remove(namespacedChildID)).To(Succeed())

					parentChildInfo := filepath.Join(baseDirectory, "garden-info", "parent-child", parentID.GraphID())
					Expect(parentChildInfo).NotTo(BeAnExistingFile())
				})

				It("should remove the child parent relationship", func() {
					Expect(aufsCake.Remove(namespacedChildID)).To(Succeed())

					childParentInfo := filepath.Join(baseDirectory, "garden-info", "child-parent", namespacedChildID.GraphID())
					Expect(childParentInfo).NotTo(BeAnExistingFile())
				})
			})

			Context("when cake remove fails", func() {
				It("should not remove the child-parent relationship file", func() {
					cake.RemoveReturns(testError)
					Expect(aufsCake.Remove(namespacedChildID)).To(Equal(testError))

					childParentInfo := filepath.Join(baseDirectory, "garden-info", "child-parent", namespacedChildID.GraphID())
					Expect(childParentInfo).To(BeAnExistingFile())
				})

				It("should not remove the parent-child relationship file", func() {
					cake.RemoveReturns(testError)
					Expect(aufsCake.Remove(namespacedChildID)).To(Equal(testError))

					parentChildInfo := filepath.Join(baseDirectory, "garden-info", "parent-child", parentID.GraphID())
					Expect(parentChildInfo).To(BeAnExistingFile())
				})
			})

		})
	})

	Describe("IsLeaf", func() {
		Context("when the docker underlying cake fails", func() {
			It("should return the error", func() {
				cake.IsLeafReturns(false, testError)

				_, err := aufsCake.IsLeaf(childID)
				Expect(err).To(Equal(testError))
			})
		})

		Context("when layer id is not valid file name", func() {
			It("should return the error", func() {
				childID.GraphIDReturns("\x00")
				cake.IsLeafReturns(true, nil)

				isLeaf, err := aufsCake.IsLeaf(childID)
				Expect(isLeaf).To(BeFalse())
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the child ID is namespaced", func() {
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

			JustBeforeEach(func() {
				cake.IsLeafStub = func(id layercake.ID) (bool, error) {
					if id == parentID {
						// as far as docker is concerned, this is a leaf, since docker
						// knows nothing about the namespaced child.
						return true, nil
					}

					if id == namespacedChildID {
						// as far as docker knows, the namespaced child has no relatives of
						// any kind
						return true, nil
					}

					// docker knows nothing about any other layers
					return false, testError
				}

				Expect(aufsCake.Create(namespacedChildID, parentID)).To(Succeed())
			})

			It("should be a leaf", func() {
				isLeaf, err := aufsCake.IsLeaf(namespacedChildID)
				Expect(err).NotTo(HaveOccurred())
				Expect(isLeaf).To(BeTrue())
			})

			It("has a non-leaf parent", func() {
				isLeaf, err := aufsCake.IsLeaf(parentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(isLeaf).To(BeFalse())
			})

			It("should persist the relationship", func() {
				otherAufsCake := &layercake.AufsCake{
					Cake:      cake,
					Runner:    runner,
					GraphRoot: baseDirectory}
				isLeaf, err := otherAufsCake.IsLeaf(parentID)
				Expect(err).NotTo(HaveOccurred())
				Expect(isLeaf).To(BeFalse())
			})
		})

		Context("when the child ID is not namespaced", func() {
			BeforeEach(func() {
				cake.IsLeafStub = func(id layercake.ID) (bool, error) {
					if id == childID {
						return true, nil
					}
					if id == parentID {
						return false, nil
					}

					return false, errors.New("Unsupported ID")
				}
			})

			It("should delegate to the cake", func() {
				isLeaf, err := aufsCake.IsLeaf(childID)
				Expect(isLeaf).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())
				Expect(cake.IsLeafCallCount()).To(Equal(1))
				Expect(cake.IsLeafArgsForCall(0)).To(Equal(childID))
			})

			It("should also delegate to the cake for the parent", func() {
				isLeaf, err := aufsCake.IsLeaf(parentID)
				Expect(isLeaf).To(BeFalse())
				Expect(err).NotTo(HaveOccurred())
				Expect(cake.IsLeafCallCount()).To(Equal(1))
				Expect(cake.IsLeafArgsForCall(0)).To(Equal(parentID))
			})
		})
	})
})
