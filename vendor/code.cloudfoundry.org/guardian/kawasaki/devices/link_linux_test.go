package devices_test

import (
	"fmt"
	"net"
	"os"
	"strings"

	"code.cloudfoundry.org/guardian/kawasaki/devices"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
)

var _ = Describe("Link Management", func() {
	var (
		l    devices.Link
		name string
		intf *net.Interface
	)

	BeforeEach(func() {
		runCommand("sh", "-c", "mountpoint /sys || mount -t sysfs sysfs /sys")

		name = fmt.Sprintf("gdn-test-%d", GinkgoParallelNode())
		link := &netlink.GenericLink{
			LinkAttrs: netlink.LinkAttrs{Name: name},
			LinkType:  "dummy",
		}

		Expect(netlink.LinkAdd(link)).To(Succeed())
		intf, _ = net.InterfaceByName(name)
	})

	AfterEach(func() {
		cleanup(name)
	})

	Describe("AddIP", func() {
		Context("when the interface exists", func() {
			It("adds the IP succesffuly", func() {
				ip, subnet, _ := net.ParseCIDR("1.2.3.4/5")
				Expect(l.AddIP(&net.Interface{Name: "something"}, ip, subnet)).To(MatchError("devices: Link not found"))
			})
		})

		Context("when the interface does not exist", func() {
			It("returns the error", func() {

				ip, subnet, _ := net.ParseCIDR("1.2.3.4/5")
				Expect(l.AddIP(intf, ip, subnet)).To(Succeed())
			})
		})
	})

	Describe("AddDefaultGW", func() {
		Context("when the interface does not exist", func() {
			It("returns the error", func() {
				ip := net.ParseIP("1.2.3.4")
				Expect(l.AddDefaultGW(&net.Interface{Name: "something"}, ip)).To(MatchError("devices: Link not found"))
			})
		})
	})

	Describe("SetUp", func() {
		Context("when the interface does not exist", func() {
			It("returns an error", func() {
				Expect(l.SetUp(&net.Interface{Name: "something"})).To(MatchError("devices: Link not found"))
			})
		})

		Context("when the interface exists", func() {
			Context("and it is down", func() {
				It("should bring the interface up", func() {
					Expect(l.SetUp(intf)).To(Succeed())

					intf, err := net.InterfaceByName(name)
					Expect(err).ToNot(HaveOccurred())
					Expect(intf.Flags & net.FlagUp).To(Equal(net.FlagUp))
				})
			})

			Context("and it is already up", func() {
				It("should still succeed", func() {
					Expect(l.SetUp(intf)).To(Succeed())
					Expect(l.SetUp(intf)).To(Succeed())

					intf, err := net.InterfaceByName(name)
					Expect(err).ToNot(HaveOccurred())
					Expect(intf.Flags & net.FlagUp).To(Equal(net.FlagUp))
				})
			})
		})
	})

	Describe("SetMTU", func() {
		Context("when the interface does not exist", func() {
			It("returns an error", func() {
				Expect(l.SetMTU(&net.Interface{Name: "something"}, 1234)).To(MatchError("devices: Link not found"))
			})
		})

		Context("when the interface exists", func() {
			It("sets the mtu", func() {
				Expect(l.SetMTU(intf, 1234)).To(Succeed())

				intf, err := net.InterfaceByName(name)
				Expect(err).ToNot(HaveOccurred())
				Expect(intf.MTU).To(Equal(1234))
			})
		})
	})

	Describe("SetNs", func() {
		var netnsName string

		BeforeEach(func() {
			netnsName = fmt.Sprintf("gdnsetnstest%d", GinkgoParallelNode())

			runCommand("sh", "-c", fmt.Sprintf("ip netns add %s", netnsName))
		})

		AfterEach(func() {
			runCommand("sh", "-c", fmt.Sprintf("ip netns delete %s", netnsName))
		})

		It("moves the interface in to the given namespace by pid", func() {
			diagnostics := func(fd uintptr, netnsfile string) string {
				var ifaceNames []string
				ifaces, err := net.Interfaces()
				Expect(err).NotTo(HaveOccurred())
				for _, iface := range ifaces {
					ifaceNames = append(ifaceNames, iface.Name)
				}

				return fmt.Sprintf(`
test outer ns interfaces: [%s]
interface we were trying to move: %s
test inner ns interfaces: %s
file descriptor of the netns: %d
ginkgo test process (pid=%d) open file descriptors: %s
processes (including threads) using  netns file %s: %s
`,
					strings.Join(ifaceNames, ", "),
					intf.Name,
					runCommand("ip", "netns", "exec", netnsName, "ip", "link"),
					fd,
					os.Getpid(), runCommand("/bin/sh", "-c", fmt.Sprintf("ls -la /proc/%d/fd", os.Getpid())),
					netnsfile, runCommand("/bin/sh", "-c", "lsof | grep netns"),
				)
			}

			// look at this perfectly ordinary hat
			cmd, _ := startCommand("ip", "netns", "exec", netnsName, "sleep", "6312736")
			defer cmd.Process.Kill()

			// (it has the following fd)
			netnsfile := fmt.Sprintf("/var/run/netns/%s", netnsName)
			f, err := os.Open(netnsfile)
			Expect(err).ToNot(HaveOccurred())
			defer f.Close()

			// I wave the magic wand
			Expect(l.SetNs(intf, int(f.Fd()))).To(Succeed(), diagnostics(f.Fd(), netnsfile))

			// the bunny has vanished! where is the bunny?
			intfs, _ := net.Interfaces()
			Expect(intfs).ToNot(ContainElement(intf))

			// oh my word it's in the hat!
			runCommand("sh", "-c", fmt.Sprintf("ip netns exec %s ifconfig %s", netnsName, name))
		})

		Context("when the interface does not exist", func() {
			It("returns the error", func() {
				Expect(l.SetNs(&net.Interface{Name: "something"}, 1234)).To(MatchError("devices: Link not found"))
			})
		})
	})

	Describe("InterfaceByName", func() {
		Context("when the interface exists", func() {
			It("returns the interface with the given name, and true", func() {
				returnedIntf, found, err := l.InterfaceByName(name)
				Expect(err).ToNot(HaveOccurred())

				Expect(returnedIntf).To(Equal(intf))
				Expect(found).To(BeTrue())
			})
		})

		Context("when the interface does not exist", func() {
			It("does not return an error", func() {
				_, found, err := l.InterfaceByName("sandwich")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("List", func() {
		It("lists all the interfaces", func() {
			names, err := l.List()
			Expect(err).ToNot(HaveOccurred())

			Expect(names).To(ContainElement(name))
		})
	})
})
