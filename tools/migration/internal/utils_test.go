package internal_test

import (
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	. "github.com/filecoin-project/go-filecoin/tools/migration/internal"
	ast "github.com/stretchr/testify/assert"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestNowString(t *testing.T) {
	tf.UnitTest(t)
	assert := ast.New(t)
	adateStr := NowString()
	rg, _ := regexp.Compile("^[0-9]{8}-[0-9]{6}$")
	assert.Regexp(rg, adateStr)
}

func TestExpandHomedir(t *testing.T) {
	tf.UnitTest(t)
	assert := ast.New(t)
	assert.Equal(ExpandHomedir("/tmp/foo"), "/tmp/foo")

	home := os.Getenv("HOME")
	expected := strings.Join([]string{home, "/foo"}, "")

	assert.Equal(expected, ExpandHomedir("~/foo"))
}
