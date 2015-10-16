package distclient_test

import (
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/docker/docker/image"

	"github.com/cloudfoundry-incubator/garden-shed/distclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

// busybox version to try to pull, should be a tag so it doesn't change
const busyBoxVersion = "1.24.0"

// expected busybox layer digests (these should never change since the tag above is locked down)
var busyBoxLayers = []image.Image{
	{
		LayerID: "sha256:df3ae2b606ca0ab01a4bc6ec2b7450a547106b47eca44a242153d3bb3fc254b9",
		ID:      "718495ebac5cf88bf00a7d01f89821cdd9f55bca7133c5766da2dac1b3a60356",
	},
	{
		LayerID: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4",
		ID:      "0064fda8c45ded4e3a4ac6d46e1f0d565697195d905960caabfe0e454f2fdfde",
		Parent:  "718495ebac5cf88bf00a7d01f89821cdd9f55bca7133c5766da2dac1b3a60356",
	},
}

var busyBoxLayerContents = [][]string{
	[]string{"bin", "dev", "etc", "home", "root", "tmp", "usr", "var"},
	[]string{},
}

var _ = Describe("distclient", func() {
	var (
		logger lager.Logger
		conn   distclient.Conn
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		var err error
		conn, err = distclient.Dial(logger, "https://registry-1.docker.io", "library/busybox")
		Expect(err).NotTo(HaveOccurred())
	})

	It("can pull a manifest from dockerhub", func() {
		layer, err := conn.GetManifest(logger, busyBoxVersion)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.Layers[0].LayerID).To(Equal(busyBoxLayers[0].LayerID))
		Expect(layer.Layers[1].LayerID).To(Equal(busyBoxLayers[1].LayerID))

		Expect(layer.Layers[0].Parent).To(Equal(busyBoxLayers[0].Parent))
		Expect(layer.Layers[1].Parent).To(Equal(busyBoxLayers[1].Parent))

		Expect(layer.Layers[0].ID).To(Equal(busyBoxLayers[0].ID))
		Expect(layer.Layers[1].ID).To(Equal(busyBoxLayers[1].ID))

		Expect(layer.Layers[0].ContainerConfig.Env).To(Equal(busyBoxLayers[0].ContainerConfig.Env))
		Expect(layer.Layers[1].ContainerConfig.Env).To(Equal(busyBoxLayers[1].ContainerConfig.Env))
	})

	It("returns bottom layer to top layer (reverse of docker api, order they should be applied to the graph)", func() {
		layer, err := conn.GetManifest(logger, busyBoxVersion)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.Layers[0].Parent).To(Equal(""))
	})

	It("can get a layer blob from dockerhub", func() {
		for i, layer := range busyBoxLayers {
			tmp := tmpDir()
			defer os.RemoveAll(tmp)

			r, err := conn.GetBlobReader(logger, layer.LayerID)
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command("tar", "zxf", "-", "-C", tmp)
			cmd.Stdin = r

			tarSession, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(tarSession, "30s").Should(gexec.Exit(0))
			Expect(fileNames(tmp)).To(ConsistOf(busyBoxLayerContents[i]))
		}
	})
})

func tmpDir() string {
	tmp, err := ioutil.TempDir("", "")
	Expect(err).NotTo(HaveOccurred())
	return tmp
}

func fileNames(path string) (names []string) {
	dir, err := ioutil.ReadDir(path)
	Expect(err).NotTo(HaveOccurred())

	for _, d := range dir {
		names = append(names, d.Name())
	}

	return
}
