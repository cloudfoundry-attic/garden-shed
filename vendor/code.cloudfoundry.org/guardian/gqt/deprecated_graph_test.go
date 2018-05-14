package gqt_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("graph flags", func() {
	var (
		client               *runner.RunningGarden
		layersPath           string
		diffPath             string
		mntPath              string
		nonDefaultRootfsPath string
		persistentImages     []string
	)

	numLayersInGraph := func() int {
		layerFiles, err := ioutil.ReadDir(layersPath)
		Expect(err).ToNot(HaveOccurred())
		diffFiles, err := ioutil.ReadDir(diffPath)
		Expect(err).ToNot(HaveOccurred())
		mntFiles, err := ioutil.ReadDir(mntPath)
		Expect(err).ToNot(HaveOccurred())

		numLayerFiles := len(layerFiles)
		Expect(numLayerFiles).To(Equal(len(diffFiles)))
		Expect(numLayerFiles).To(Equal(len(mntFiles)))
		return numLayerFiles
	}

	expectLayerCountAfterGraphCleanupToBe := func(layerCount int) {
		nonPersistantRootfsContainer, err := client.Create(garden.ContainerSpec{
			RootFSPath: nonDefaultRootfsPath,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(client.Destroy(nonPersistantRootfsContainer.Handle())).To(Succeed())
		Expect(numLayersInGraph()).To(Equal(layerCount + 2)) // +2 for the layers created for the nondefaultrootfs container
	}

	peaDoesNotLeaveDebrisInGraphOnExit := func(imageURI string) {
		var (
			pea                             garden.Process
			numLayersAfterContainerCreation int
		)

		JustBeforeEach(func() {
			imageURI := imageURI
			container, err := client.Create(garden.ContainerSpec{
				RootFSPath: imageURI,
			})
			Expect(err).ToNot(HaveOccurred())

			numLayersAfterContainerCreation = numLayersInGraph()

			pea, err = container.Run(garden.ProcessSpec{
				Path:  "/bin/sh",
				Args:  []string{"-c", "exit 0"},
				Image: garden.ImageRef{URI: imageURI},
			}, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not leave debris in the graph on exit", func() {
			Expect(pea.Wait()).To(Equal(0))

			numLayersAfterPeaExits := numLayersInGraph()
			Expect(numLayersAfterPeaExits).To(Equal(numLayersAfterContainerCreation))
		})
	}

	BeforeEach(func() {
		var err error
		nonDefaultRootfsPath, err = ioutil.TempDir("", "tmpRootfs")
		Expect(err).ToNot(HaveOccurred())
		// temporary workaround as runc expects a /tmp dir to exist in the container rootfs
		err = os.Mkdir(filepath.Join(nonDefaultRootfsPath, "tmp"), 0700)
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		config.PersistentImages = persistentImages
		config = resetImagePluginConfig()
		client = runner.Start(config)

		layersPath = path.Join(*client.GraphDir, "aufs", "layers")
		diffPath = path.Join(*client.GraphDir, "aufs", "diff")
		mntPath = path.Join(*client.GraphDir, "aufs", "mnt")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(nonDefaultRootfsPath)).To(Succeed())
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Describe("--graph-cleanup-threshold-in-megabytes", func() {
		JustBeforeEach(func() {
			Expect(numLayersInGraph()).To(Equal(0))
			container, err := client.Create(garden.ContainerSpec{
				RootFSPath: "docker:///busybox",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(client.Destroy(container.Handle())).To(Succeed())
		})

		Context("when the graph cleanup threshold is set to -1", func() {
			BeforeEach(func() {
				config.GraphCleanupThresholdMB = intptr(-1)
			})

			It("does NOT clean up the graph directory on create", func() {
				initialNumberOfLayers := numLayersInGraph()
				anotherContainer, err := client.Create(garden.ContainerSpec{})
				Expect(err).ToNot(HaveOccurred())

				Expect(numLayersInGraph()).To(BeNumerically(">", initialNumberOfLayers), "after creation, should NOT have deleted anything")
				Expect(client.Destroy(anotherContainer.Handle())).To(Succeed())
			})

			Context("and we run a pea with the same image as the container", func() {
				peaDoesNotLeaveDebrisInGraphOnExit("docker:///busybox")
			})

			Context("and we run a pea with another image", func() {
				peaDoesNotLeaveDebrisInGraphOnExit("docker:///alpine")
			})
		})

		Context("when the graph cleanup threshold is set to 0", func() {
			BeforeEach(func() {
				config.GraphCleanupThresholdMB = intptr(0)
			})

			Context("and we run a pea with the same image as the container", func() {
				peaDoesNotLeaveDebrisInGraphOnExit("docker:///busybox")
			})

			Context("and we run a pea with another image", func() {
				peaDoesNotLeaveDebrisInGraphOnExit("docker:///alpine")
			})
		})

		Context("when the graph cleanup threshold is exceeded", func() {
			BeforeEach(func() {
				config.GraphCleanupThresholdMB = intptr(0)
				persistentImages = []string{}
			})

			Context("when there are other rootfs layers in the graph dir", func() {
				It("cleans up the graph directory on container creation (and not on destruction)", func() {
					Expect(numLayersInGraph()).To(BeNumerically(">", 0))

					anotherContainer, err := client.Create(garden.ContainerSpec{})
					Expect(err).ToNot(HaveOccurred())

					Expect(numLayersInGraph()).To(Equal(3), "after creation, should have deleted everything other than the default rootfs, uid translation layer and container layer")
					Expect(client.Destroy(anotherContainer.Handle())).To(Succeed())
					Expect(numLayersInGraph()).To(Equal(2), "should not garbage collect parent layers on destroy")
				})
			})
		})

		Context("when the graph cleanup threshold is not exceeded", func() {
			BeforeEach(func() {
				config.GraphCleanupThresholdMB = intptr(1024)
			})

			It("does not cleanup", func() {
				// threshold is not yet exceeded
				Expect(numLayersInGraph()).To(Equal(3))

				anotherContainer, err := client.Create(garden.ContainerSpec{})
				Expect(err).ToNot(HaveOccurred())

				Expect(numLayersInGraph()).To(Equal(6))
				Expect(client.Destroy(anotherContainer.Handle())).To(Succeed())
			})
		})
	})

	Describe("--persistentImage", func() {
		BeforeEach(func() {
			config.GraphCleanupThresholdMB = intptr(0)
		})

		Context("when set", func() {
			JustBeforeEach(func() {
				Eventually(client, "60s").Should(gbytes.Say("retain.retained"))
			})

			Context("and local images are used", func() {
				BeforeEach(func() {
					persistentImages = []string{defaultTestRootFS}
				})

				Describe("graph cleanup for a rootfs on the whitelist", func() {
					It("keeps the rootfs in the graph", func() {
						container, err := client.Create(garden.ContainerSpec{
							RootFSPath: persistentImages[0],
						})
						Expect(err).ToNot(HaveOccurred())
						Expect(client.Destroy(container.Handle())).To(Succeed())

						expectLayerCountAfterGraphCleanupToBe(2)
					})

					Context("which is a symlink", func() {
						BeforeEach(func() {
							Expect(os.MkdirAll("/var/vcap/packages", 0755)).To(Succeed())
							err := exec.Command("ln", "-s", defaultTestRootFS, "/var/vcap/packages/busybox").Run()
							Expect(err).ToNot(HaveOccurred())

							persistentImages = []string{"/var/vcap/packages/busybox"}
						})

						AfterEach(func() {
							Expect(os.RemoveAll("/var/vcap/packages")).To(Succeed())
						})

						It("keeps the rootfs in the graph", func() {
							container, err := client.Create(garden.ContainerSpec{
								RootFSPath: persistentImages[0],
							})
							Expect(err).ToNot(HaveOccurred())
							Expect(client.Destroy(container.Handle())).To(Succeed())

							expectLayerCountAfterGraphCleanupToBe(2)
						})
					})
				})

				Describe("graph cleanup for a rootfs not on the whitelist", func() {
					It("cleans up all unused images from the graph", func() {
						container, err := client.Create(garden.ContainerSpec{
							RootFSPath: nonDefaultRootfsPath,
						})
						Expect(err).ToNot(HaveOccurred())
						Expect(client.Destroy(container.Handle())).To(Succeed())

						expectLayerCountAfterGraphCleanupToBe(0)
					})
				})
			})

			Context("and docker images are used", func() {
				BeforeEach(func() {
					persistentImages = []string{
						"docker:///busybox",
						"docker:///ubuntu",
						"docker://banana/bananatest",
					}
				})

				Describe("graph cleanup for a rootfs on the whitelist", func() {
					It("keeps the rootfs in the graph", func() {
						numLayersBeforeDockerPull := numLayersInGraph()
						container, err := client.Create(garden.ContainerSpec{
							RootFSPath: persistentImages[0],
						})
						Expect(err).ToNot(HaveOccurred())
						Expect(client.Destroy(container.Handle())).To(Succeed())
						numLayersInImage := numLayersInGraph() - numLayersBeforeDockerPull

						expectLayerCountAfterGraphCleanupToBe(numLayersInImage)
					})
				})

				Describe("graph cleanup for a rootfs not on the whitelist", func() {
					It("cleans up all unused images from the graph", func() {
						container, err := client.Create(garden.ContainerSpec{
							RootFSPath: "docker:///cfgarden/garden-busybox",
						})
						Expect(err).ToNot(HaveOccurred())
						Expect(client.Destroy(container.Handle())).To(Succeed())

						expectLayerCountAfterGraphCleanupToBe(0)
					})
				})
			})
		})

		Context("when it is not set", func() {
			BeforeEach(func() {
				persistentImages = []string{}
			})

			It("cleans up all unused images from the graph", func() {
				defaultRootfsContainer, err := client.Create(garden.ContainerSpec{})
				Expect(err).ToNot(HaveOccurred())

				nonDefaultRootfsContainer, err := client.Create(garden.ContainerSpec{
					RootFSPath: nonDefaultRootfsPath,
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(client.Destroy(defaultRootfsContainer.Handle())).To(Succeed())
				Expect(client.Destroy(nonDefaultRootfsContainer.Handle())).To(Succeed())

				expectLayerCountAfterGraphCleanupToBe(0)
			})
		})
	})
})
