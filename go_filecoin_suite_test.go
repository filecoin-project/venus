package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGoFilecoin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GoFilecoin Suite")
}
