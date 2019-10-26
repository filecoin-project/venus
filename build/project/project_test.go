package project

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIAmRoot(t *testing.T) {
	assert.Equal(t, Root(), Root())
}

func TestRootOptions(t *testing.T) {
	root := Root()
	gfc := Root("go-filecoin")

	expected := fmt.Sprintf("%s/%s", root, "go-filecoin")
	assert.Equal(t, gfc, expected)
}
