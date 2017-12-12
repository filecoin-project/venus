package main_test

import (
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/filecoin-project/go-filecoin"
)

func TestHello(t *testing.T) {
	RegisterTestingT(t)

	Expect(PrintVersion()).To(HavePrefix("Hello Filecoin: "))
}
