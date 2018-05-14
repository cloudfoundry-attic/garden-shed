package dns_test

import (
	"fmt"
	"net"

	. "code.cloudfoundry.org/guardian/kawasaki/dns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("compiling the resolv.conf file", func() {
	var (
		determiner = &ResolvCompiler{}
		hostIP     = net.ParseIP("9.8.7.6")
	)

	DescribeTable(
		"Determine",
		func(hostResolvContents string, pluginNameservers, operatorNameservers, additionalNameservers []net.IP, pluginSearchDomains, expectedEntries []string) {
			actualEntries := determiner.Determine(hostResolvContents, hostIP, pluginNameservers, operatorNameservers, additionalNameservers, pluginSearchDomains)
			Expect(actualEntries).To(Equal(expectedEntries))
		},
		Entry(
			"when passed >=1 pluginNameservers, it returns only them",
			"nameserver 1.2.3.4\n", ips("10.0.0.3"), ips("10.0.0.1", "10.0.0.2"), ips("10.0.0.4"), nil,
			nameservers("10.0.0.3"),
		),
		Entry(
			"when passed >=1 pluginNameservers and >=1 pluginSearchDomains, it returns them",
			"nameserver 1.2.3.4\n", ips("10.0.0.3"), ips("10.0.0.1", "10.0.0.2"), ips("10.0.0.4"), []string{"one", "two"},
			append(nameservers("10.0.0.3"), "search one two"),
		),
		Entry(
			"when explicitly passed 0 pluginNameservers, it returns an empty list",
			"nameserver 1.2.3.4\n", ips(), ips("10.0.0.1", "10.0.0.2"), ips("10.0.0.4"), nil,
			nameservers(),
		),
		Entry(
			"when passed >=1 operatorNameservers, it returns them",
			"nameserver 1.2.3.4\n", nil, ips("10.0.0.1", "10.0.0.2"), ips(), nil,
			nameservers("10.0.0.1", "10.0.0.2"),
		),
		Entry(
			"when passed >=1 operatorNameservers and >=1 pluginSearchDomains, it returns them",
			"nameserver 1.2.3.4\n", nil, ips("10.0.0.1", "10.0.0.2"), ips(), []string{"one", "two"},
			append(nameservers("10.0.0.1", "10.0.0.2"), "search one two"),
		),
		Entry("when passed >=1 additionalNameservers and 0 operatorNameservers, it appends them to the host's nameservers",
			"nameserver 1.2.3.4\narbitrary text\n", nil, nil, ips("10.0.0.1", "10.0.0.2"), nil,
			[]string{"nameserver 1.2.3.4", "arbitrary text", "nameserver 10.0.0.1", "nameserver 10.0.0.2"},
		),
		Entry("when the host nameservers contain loopback (127.0.0.0/8) entries, it returns all other entries",
			"nameserver 1.2.3.4\nnameserver 127.0.0.19\nnameserver 127.0.18.19\nnameserver 127.17.0.19\n", nil, nil, ips(), nil,
			nameservers("1.2.3.4"),
		),
		Entry("when the host nameservers consist of exactly one loopback entry, it returns the host IP",
			"nameserver 127.0.0.19\n", nil, nil, ips(), nil,
			nameservers(hostIP.String()),
		),
		Entry("when passed >=1 additionalNameservers and >1 operatorNameservers, it returns those lists and nothing from host",
			"nameserver 1.2.3.4\n", nil, ips("10.0.0.3"), ips("10.0.0.1", "10.0.0.2"), nil,
			nameservers("10.0.0.3", "10.0.0.1", "10.0.0.2"),
		),
		Entry("when passed 0 pluginNameservers, 0 operatorNameservers and 0 pluginSearchDomains it returns the host's non-127.0.0.0/8 nameservers and search domains",
			"nameserver 1.2.3.4\nnameserver 127.0.0.1\nsearch hey", nil, nil, ips(), nil,
			append(nameservers("1.2.3.4"), "search hey"),
		),
		Entry("when passed 0 pluginNameservers, 0 operatorNameservers and >=1 pluginSearchDomains it returns the host's non-127.0.0.0/8 nameservers and the pluginSearchDomains",
			"nameserver 1.2.3.4\nnameserver 127.0.0.1\nsearch hey", nil, nil, ips(), []string{"ho"},
			append(nameservers("1.2.3.4"), "search ho"),
		),
		Entry("when passed 0 additionalNameservers and 0 operatorNameservers, it returns the host's non-127.0.0.0/8 nameservers",
			"nameserver 1.2.3.4\nnameserver 127.0.0.1\nnameserver 127.0.2.1\nnameserver 5.6.7.8\n", nil, nil, ips(), nil,
			nameservers("1.2.3.4", "5.6.7.8"),
		),
		Entry("when the host has no nameservers, it returns an empty list",
			"", nil, nil, ips(), nil,
			nameservers(),
		),
		Entry("when the host has non-nameserver entries, it returns them",
			"arbitrary text\n", nil, nil, nil, nil,
			[]string{"arbitrary text"},
		),
		Entry("when the host has non-nameserver entries, and any loopback nameservers, it returns only the non-nameserver entries",
			"nameserver 127.0.0.1\narbitrary text\n", nil, nil, nil, nil,
			[]string{"arbitrary text"},
		),
	)
})

func nameservers(ips ...string) []string {
	entries := []string{}
	for _, ip := range ips {
		entries = append(entries, fmt.Sprintf("nameserver %s", ip))
	}

	return entries
}
