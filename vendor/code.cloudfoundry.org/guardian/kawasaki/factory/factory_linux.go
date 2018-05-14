package factory

import (
	"os"

	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/kawasaki/configure"
	"code.cloudfoundry.org/guardian/kawasaki/devices"
	"code.cloudfoundry.org/guardian/kawasaki/dns"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"code.cloudfoundry.org/guardian/kawasaki/netns"
)

func NewDefaultConfigurer(ipt *iptables.IPTablesController, depotDir string) kawasaki.Configurer {
	resolvConfigurer := &kawasaki.ResolvConfigurer{
		HostsFileCompiler: &dns.HostsFileCompiler{},
		ResolvCompiler:    &dns.ResolvCompiler{},
		DepotDir:          depotDir,
		ResolvFilePath:    "/etc/resolv.conf",
	}

	hostConfigurer := &configure.Host{
		Veth:       &devices.VethCreator{},
		Link:       &devices.Link{},
		Bridge:     &devices.Bridge{},
		FileOpener: netns.Opener(os.Open),
	}

	containerConfigurer := &configure.Container{
		FileOpener: netns.Opener(os.Open),
	}

	return kawasaki.NewConfigurer(
		resolvConfigurer,
		hostConfigurer,
		containerConfigurer,
		iptables.NewInstanceChainCreator(ipt),
	)
}
