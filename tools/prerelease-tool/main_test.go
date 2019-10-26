package main

import (
	"context"
	"strconv"
	"testing"
	"time"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/google/go-github/github"
	"github.com/stretchr/testify/assert"
)

func mock() *prereleaseTool {
	r := prereleaseTool{
		Limit:  2,
		DryRun: true,
	}
	now := time.Now()
	name1 := "release"
	prerelease1 := false
	id1 := int64(1)
	tag1 := "v" + strconv.FormatInt(id1, 10)
	time1 := now.Add(-1 * time.Minute)
	r.Releases = append(r.Releases, &github.RepositoryRelease{
		Name:       &name1,
		Prerelease: &prerelease1,
		ID:         &id1,
		CreatedAt:  &github.Timestamp{Time: time1},
		TagName:    &tag1,
	})
	name2 := "prerelease-middle"
	prerelease2 := true
	id2 := int64(2)
	tag2 := "v" + strconv.FormatInt(id2, 10)
	time2 := now.Add(-3 * time.Minute)
	r.Releases = append(r.Releases, &github.RepositoryRelease{
		Name:       &name2,
		Prerelease: &prerelease2,
		ID:         &id2,
		CreatedAt:  &github.Timestamp{Time: time2},
		TagName:    &tag2,
	})
	name3 := "prerelease-oldest"
	prerelease3 := true
	id3 := int64(3)
	tag3 := "v" + strconv.FormatInt(id3, 10)
	time3 := now.Add(-4 * time.Minute)
	r.Releases = append(r.Releases, &github.RepositoryRelease{
		Name:       &name3,
		Prerelease: &prerelease3,
		ID:         &id3,
		CreatedAt:  &github.Timestamp{Time: time3},
		TagName:    &tag3,
	})
	name4 := "prerelease-newest"
	prerelease4 := true
	id4 := int64(4)
	tag4 := "v" + strconv.FormatInt(id4, 10)
	time4 := now.Add(-2 * time.Minute)
	r.Releases = append(r.Releases, &github.RepositoryRelease{
		Name:       &name4,
		Prerelease: &prerelease4,
		ID:         &id4,
		CreatedAt:  &github.Timestamp{Time: time4},
		TagName:    &tag4,
	})
	return &r
}

func TestGetPrereleases(t *testing.T) {
	tf.UnitTest(t)
	r := mock()
	ok := r.getPrereleases()
	assert.True(t, ok, "releases should be found and this be true")
	assert.Len(t, r.Prereleases, 3, "Prelease count is wrong")
	for _, release := range r.Prereleases {
		assert.NotEqual(t, *release.Name, "release", "Not a Pre-Release")
	}
	assert.Equal(t, *r.Prereleases[0].Name, "prerelease-newest", "Wrong Name at first index")
	assert.Equal(t, *r.Prereleases[1].Name, "prerelease-middle", "Wrong Name at middle index")
	assert.Equal(t, *r.Prereleases[2].Name, "prerelease-oldest", "Wrong Name at last index")
}

func TestGetPrereleasesBoundsCheck(t *testing.T) {
	tf.UnitTest(t)
	r := prereleaseTool{}
	ok := r.getPrereleases()
	assert.False(t, ok, "releases should be found and this be true")
	assert.Len(t, r.Prereleases, 0, "Prelease count is wrong")
}

func TestOutdatedPreleaseIDs(t *testing.T) {
	tf.UnitTest(t)
	r := mock()
	r.getPrereleases()
	outdated := r.outdatedPreleaseIDs()
	assert.Len(t, outdated, 1, "There should only be one ID")
	t.Logf("outdated: %+v", outdated)
	tag, ok := outdated[3]
	assert.True(t, ok, "map key should exist")
	assert.Equal(t, tag, "v3", "unexpected prerelease name")
}

func TestOutdatedPreleaseIDsBoundsCheck(t *testing.T) {
	tf.UnitTest(t)
	r := prereleaseTool{}
	r.getPrereleases()
	oldIDs := r.outdatedPreleaseIDs()
	assert.Len(t, oldIDs, 0, "There should only be no IDs")
}

func TestTrim(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	r := mock()
	r.getPrereleases()
	ok, err := r.deleteReleases(ctx, r.outdatedPreleaseIDs())
	assert.False(t, ok, "This was a dry run, ok should be false")
	assert.NoError(t, err, "No error should exist")
}

func TestTrimBoundsCheck(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	r := prereleaseTool{
		DryRun: true,
	}
	r.getPrereleases()
	ok, err := r.deleteReleases(ctx, r.outdatedPreleaseIDs())
	assert.False(t, ok, "This was a dry run, ok should be false")
	assert.NoError(t, err, "No error should exist")
}
