// +build e2e

package utils_signal_test

import (
	"os"
	"os/exec"
	"syscall"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "monitor pods suite")
}

var utilsSignalBinPath = ""

var _ = SynchronizedBeforeSuite(func() []byte {
	binPath, err := gexec.Build("./testdata/utils_signal", "--tags", "test")
	Ω(err).ShouldNot(HaveOccurred())
	return []byte(binPath)
}, func(binPath []byte) {
	utilsSignalBinPath = string(binPath)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	_ = os.Remove(utilsSignalBinPath)
})

var _ = Describe("Process blocked by WaitForProcessInterruption", func() {
	Context("without callback", func() {
		var session *gexec.Session

		BeforeEach(func() {
			var err error
			cmd := exec.Command(utilsSignalBinPath, "nocb")
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gbytes.Say("Start"))
		})

		It("should return 128+2 on single SIGINT", func() {
			session.Signal(os.Interrupt)
			Eventually(session).Should(gbytes.Say("Forced"))
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode()).To(Equal(128 + int(syscall.SIGINT)))
		})

		It("should return 128+15 on single SIGTERM", func() {
			session.Signal(syscall.SIGTERM)
			Eventually(session).Should(gbytes.Say("Forced"))
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode()).To(Equal(128 + int(syscall.SIGTERM)))
		})

	})

	Context("with callback", func() {
		var session *gexec.Session

		BeforeEach(func() {
			var err error
			cmd := exec.Command(utilsSignalBinPath, "withcb")
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gbytes.Say("Start"))
		})

		It("should return 0 on single SIGINT", func() {
			session.Signal(os.Interrupt)
			Eventually(session).Should(gbytes.Say("Grace"))
			Eventually(session).ShouldNot(gbytes.Say("Forced"))

			Eventually(session).Should(gexec.Exit())

			Expect(session.ExitCode()).To(Equal(0))
		})

		It("should return 0 on single SIGTERM", func() {
			session.Signal(syscall.SIGTERM)
			Eventually(session).Should(gbytes.Say("Grace"))
			Eventually(session).ShouldNot(gbytes.Say("Forced"))

			Eventually(session).Should(gexec.Exit())

			Expect(session.ExitCode()).To(Equal(0))
		})
	})

	Context("with long callback", func() {
		var session *gexec.Session

		BeforeEach(func() {
			var err error
			cmd := exec.Command(utilsSignalBinPath, "withlongcb")
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("Should return 128+2 on double SIGINT", func() {
			Eventually(session).Should(gbytes.Say("Start"))
			session.Signal(os.Interrupt)
			Eventually(session).Should(gbytes.Say("Grace"))
			session.Signal(os.Interrupt)
			Eventually(session).Should(gbytes.Say("Forced"))

			Eventually(session).Should(gexec.Exit())

			Expect(session.ExitCode()).To(Equal(128 + int(syscall.SIGINT)))
		})

		It("Should return 128+15 on double SIGTERM", func() {
			Eventually(session).Should(gbytes.Say("Start"))
			session.Signal(syscall.SIGTERM)
			Eventually(session).Should(gbytes.Say("Grace"))
			session.Signal(syscall.SIGTERM)
			Eventually(session).Should(gbytes.Say("Forced"))

			Eventually(session).Should(gexec.Exit())

			Expect(session.ExitCode()).To(Equal(128 + int(syscall.SIGTERM)))
		})

		It("Should return 128+15 on SIGINT + SIGTERM", func() {
			Eventually(session).Should(gbytes.Say("Start"))
			session.Signal(syscall.SIGINT)
			Eventually(session).Should(gbytes.Say("Grace"))
			session.Signal(syscall.SIGTERM)
			Eventually(session).Should(gbytes.Say("Forced"))

			Eventually(session).Should(gexec.Exit())

			Expect(session.ExitCode()).To(Equal(128 + int(syscall.SIGTERM)))
		})
	})
})
