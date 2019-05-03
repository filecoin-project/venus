package internal_test

import (
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

func TestNowString(t *testing.T) {
	tf.UnitTest(t)
	adateStr := NowString()
	rg, _ := regexp.Compile("^[0-9]{8}-[0-9]{6}$")
	assert.Regexp(t, rg, adateStr)
}

func SomeTestHelper() string {
	return "helper"
}
