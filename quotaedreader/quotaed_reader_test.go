package quotaedreader_test

import (
	"code.cloudfoundry.org/garden-shed/quotaedreader"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"io"
	"io/ioutil"
	"strings"
)

var _ = Describe("QuotaedReader", func() {
	var (
		delegate io.Reader
		quota    int64
		qr       *quotaedreader.QuotaedReader
	)

	BeforeEach(func() {
		quota = 20
	})

	JustBeforeEach(func() {
		qr = quotaedreader.New(ioutil.NopCloser(delegate), quota)
	})

	Describe("Read", func() {
		Context("when the underlying reader has less bytes than the quota", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("hello")
			})

			It("reads all the data", func() {
				Expect(ioutil.ReadAll(qr)).To(Equal([]byte("hello")))
			})
		})

		Context("when the underlying reader has just as many bytes as the quota", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("12345678901234567890")
			})

			It("reads all the data", func() {
				Expect(ioutil.ReadAll(qr)).To(Equal([]byte("12345678901234567890")))
			})

			It("bytes remaining to quota are zero", func() {
				ioutil.ReadAll(qr)
				Expect(qr.QuotaLeft).To(BeZero())
			})
		})

		Context("when the underlying reader has more bytes than the quota", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("blah blah blah blah blah blah blah blah")
			})

			It("returns an error", func() {
				_, err := ioutil.ReadAll(qr)
				Expect(err).To(MatchError("layer size exceeds image quota"))
			})

			It("reads only as many bytes as allowed by the quota plus one", func() {
				b, _ := ioutil.ReadAll(qr)
				Expect(b).To(HaveLen(int(quota + 1)))
			})
		})

		Context("when we pass a negative quota", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("OMG, negative quota!")
				quota = -1
			})

			It("returns an error", func() {
				_, err := ioutil.ReadAll(qr)
				Expect(err).To(MatchError("layer size exceeds image quota"))
			})
		})

		Context("when we pass zero quota", func() {
			BeforeEach(func() {
				delegate = strings.NewReader("A")
				quota = 0
			})

			It("returns an error", func() {
				_, err := ioutil.ReadAll(qr)
				Expect(err).To(MatchError("layer size exceeds image quota"))
			})
		})
	})
})
