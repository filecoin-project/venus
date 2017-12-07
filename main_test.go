package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/filecoin-project/go-filecoin"
)

var _ = Describe("Main", func() {
	It("should print hello", func() {
		Expect(PrintVersion()).To(HavePrefix("Hello Filecoin: "))
	})
})
