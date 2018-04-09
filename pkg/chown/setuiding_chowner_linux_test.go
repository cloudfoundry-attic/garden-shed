package chown_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"code.cloudfoundry.org/garden-shed/pkg/chown"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"golang.org/x/sys/unix"
)

var _ = Describe("chowning", func() {
	Context("when the file is a regular file", func() {
		var someFile string

		BeforeEach(func() {
			f, err := ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())
			someFile = f.Name()

			sess, err := gexec.Start(exec.Command("chmod", "u+s", someFile), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			info, err := os.Stat(someFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(info.Mode() & os.ModeSetuid).ToNot(Equal(os.FileMode(0)))

			Expect(chown.Chown(someFile, 100, 100)).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.Remove(someFile)).To(Succeed())
		})

		It("maintains the setuid bit", func() {
			info, err := os.Stat(someFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(info.Mode() & os.ModeSetuid).ToNot(Equal(os.FileMode(0)))
		})

		It("changes the gid and uid", func() {
			var stat unix.Stat_t
			Expect(unix.Stat(someFile, &stat)).To(Succeed())
			Expect(stat.Uid).To(BeEquivalentTo(100))
			Expect(stat.Gid).To(BeEquivalentTo(100))
		})
	})

	Context("when the file is a symlink", func() {
		var symlink string
		var target string

		BeforeEach(func() {
			f, err := ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())
			target = f.Name()

			symlinkDir, err := ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			symlink = path.Join(symlinkDir, "link")
			Expect(os.Symlink(target, symlink)).To(Succeed())

			sess, err := gexec.Start(exec.Command("chmod", "u+s", target), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			Expect(chown.Chown(symlink, 100, 100)).To(Succeed())
		})

		It("does not change the gid/uid of the target", func() {
			var stat unix.Stat_t
			Expect(unix.Stat(target, &stat)).To(Succeed())
			Expect(stat.Uid).NotTo(BeEquivalentTo(100))
			Expect(stat.Gid).NotTo(BeEquivalentTo(100))
		})

		It("avoids accidentally clobbering the mode of the target (symlinks dont have modes, but we should avoid clobbering the target)", func() {
			info, err := os.Stat(target)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode() & os.ModeSetuid).NotTo(Equal(os.FileMode(0)))
		})
	})
})
