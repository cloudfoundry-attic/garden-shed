package aufs_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cloudfoundry-incubator/garden-shed/docker_drivers/aufs"
	"github.com/cloudfoundry-incubator/garden-shed/pkg/retrier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("LoopLinux", func() {
	var (
		bsFilePath string
		destPath   string
		loop       *aufs.Loop
	)

	BeforeEach(func() {
		var err error

		tempFile, err := ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())
		bsFilePath = tempFile.Name()
		_, err = exec.Command("truncate", "-s", "10M", bsFilePath).CombinedOutput()
		Expect(err).NotTo(HaveOccurred())
		_, err = exec.Command("mkfs.ext4", "-F", bsFilePath).CombinedOutput()
		Expect(err).NotTo(HaveOccurred())

		destPath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		loop = &aufs.Loop{
			Logger: lagertest.NewTestLogger("test"),
			Retrier: &retrier.Retrier{
				Timeout:         10 * time.Second,
				PollingInterval: 50 * time.Millisecond,
				Clock:           clock.NewClock(),
			},
		}
	})

	AfterEach(func() {
		syscall.Unmount(destPath, 0)
		Expect(os.RemoveAll(destPath)).To(Succeed())
		Expect(os.Remove(bsFilePath)).To(Succeed())
	})

	Describe("MountFile", func() {
		It("mounts the file", func() {
			Expect(loop.MountFile(bsFilePath, destPath)).To(Succeed())

			session, err := gexec.Start(exec.Command("mount"), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gbytes.Say(
				fmt.Sprintf("%s on %s type ext4 \\(rw\\)", bsFilePath, destPath),
			))
		})

		Context("when using a file that does not exist", func() {
			It("should return an error", func() {
				Expect(loop.MountFile("/path/to/my/nonexisting/banana", "/path/to/dest")).To(HaveOccurred())
			})
		})
	})

	Describe("Unmount", func() {
		It("should not leak devices", func() {
			var devicesAfterCreate, devicesAfterRelease int

			destPaths := make([]string, 10)
			for i := 0; i < 10; i++ {
				var err error

				tempFile, err := ioutil.TempFile("", "")
				Expect(err).NotTo(HaveOccurred())

				_, err = exec.Command("truncate", "-s", "10M", tempFile.Name()).CombinedOutput()
				Expect(err).NotTo(HaveOccurred())
				_, err = exec.Command("mkfs.ext4", "-F", tempFile.Name()).CombinedOutput()
				Expect(err).NotTo(HaveOccurred())

				destPaths[i], err = ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())

				Expect(loop.MountFile(tempFile.Name(), destPaths[i])).To(Succeed())
			}

			output, err := exec.Command("sh", "-c", "losetup -a | wc -l").CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			devicesAfterCreate, err = strconv.Atoi(strings.TrimSpace(string(output)))
			Expect(err).NotTo(HaveOccurred())

			for i := 0; i < 10; i++ {
				Expect(loop.Unmount(destPaths[i])).To(Succeed())
			}

			output, err = exec.Command("sh", "-c", "losetup -a | wc -l").CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			devicesAfterRelease, err = strconv.Atoi(strings.TrimSpace(string(output)))
			Expect(err).NotTo(HaveOccurred())

			Expect(devicesAfterRelease).To(BeNumerically("~", devicesAfterCreate-10, 2))
		})

		It("should try to unmount multiple times", func() {
			Expect(loop.MountFile(bsFilePath, destPath)).To(Succeed())
			testFile, err := ioutil.TempFile(destPath, "")
			Expect(err).NotTo(HaveOccurred())

			c := make(chan struct{})

			go func(c chan struct{}, destPath string) {
				defer GinkgoRecover()

				Expect(loop.Unmount(destPath)).To(Succeed())

				close(c)
			}(c, destPath)

			time.Sleep(time.Millisecond * 100)
			Expect(testFile.Close()).To(Succeed())

			Eventually(c).Should(BeClosed())
		})

		Context("when the provided mount point does not exist", func() {
			It("should succeed", func() {
				Expect(loop.Unmount("/dev/loopbanana")).To(Succeed())
			})
		})
	})
})
