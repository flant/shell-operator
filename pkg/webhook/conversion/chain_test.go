package conversion

import (
	"testing"

	. "github.com/onsi/gomega"
)

func Test_VersionsMatched(t *testing.T) {
	g := NewWithT(t)
	var v0 string
	var v1 string
	var res bool

	v0 = "v1beta1"
	v1 = "v1beta1"
	res = VersionsMatched(v0, v1)
	g.Expect(res).Should(BeTrue(), "Expect that '%s' is matching '%s'.")

	v0 = "a/v1beta1"
	v1 = "v1beta1"
	res = VersionsMatched(v0, v1)
	g.Expect(res).Should(BeTrue(), "Expect that '%s' is matching '%s'.")

	v0 = "a/v1beta1"
	v1 = "a/v1beta1"
	res = VersionsMatched(v0, v1)
	g.Expect(res).Should(BeTrue(), "Expect that '%s' is matching '%s'.")

	v0 = "v1beta1"
	v1 = "a/v1beta1"
	res = VersionsMatched(v0, v1)
	g.Expect(res).Should(BeTrue(), "Expect that '%s' is matching '%s'.")

	// Negative
	v0 = "b/v1beta1"
	v1 = "a/v1beta1"
	res = VersionsMatched(v0, v1)
	g.Expect(res).Should(BeFalse(), "Expect that '%s' is not matching '%s'.")
}
