package main

import (
	"context"
	"testing"
	"time"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
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
	time1 := now.Add(-1 * time.Minute)
	r.Data = append(r.Data, &github.RepositoryRelease{
		Name:       &name1,
		Prerelease: &prerelease1,
		ID:         &id1,
		CreatedAt:  &github.Timestamp{time1},
	})
	name2 := "prerelease-middle"
	prerelease2 := true
	id2 := int64(2)
	time2 := now.Add(-3 * time.Minute)
	r.Data = append(r.Data, &github.RepositoryRelease{
		Name:       &name2,
		Prerelease: &prerelease2,
		ID:         &id2,
		CreatedAt:  &github.Timestamp{time2},
	})
	name3 := "prerelease-oldest"
	prerelease3 := true
	id3 := int64(3)
	time3 := now.Add(-4 * time.Minute)
	r.Data = append(r.Data, &github.RepositoryRelease{
		Name:       &name3,
		Prerelease: &prerelease3,
		ID:         &id3,
		CreatedAt:  &github.Timestamp{time3},
	})
	name4 := "prerelease-newest"
	prerelease4 := true
	id4 := int64(4)
	time4 := now.Add(-2 * time.Minute)
	r.Data = append(r.Data, &github.RepositoryRelease{
		Name:       &name4,
		Prerelease: &prerelease4,
		ID:         &id4,
		CreatedAt:  &github.Timestamp{time4},
	})
	return &r
}

func TestGetPrereleases(t *testing.T) {
	tf.UnitTest(t)
	r := mock()
	r.getPrereleases()
	assert.Len(t, r.Prereleases, 3, "Prelease count is wrong")
	for _, release := range r.Prereleases {
		assert.NotEqual(t, *release.Name, "release", "Not a Pre-Release")
	}
}

func TestGetPrereleasesBoundsCheck(t *testing.T) {
	tf.UnitTest(t)
	r := prereleaseTool{}
	r.getPrereleases()
	assert.Len(t, r.Prereleases, 0, "Prelease count is wrong")
}

func TestSortReleasesByDate(t *testing.T) {
	tf.UnitTest(t)
	r := mock()
	r.getPrereleases()
	r.sortReleasesByDate()
	assert.Equal(t, *r.Prereleases[0].Name, "prerelease-newest", "Wrong Name at first index")
	assert.Equal(t, *r.Prereleases[1].Name, "prerelease-middle", "Wrong Name at middle index")
	assert.Equal(t, *r.Prereleases[2].Name, "prerelease-oldest", "Wrong Name at last index")
}

func TestSortReleasesByDateBoundsCheck(t *testing.T) {
	tf.UnitTest(t)
	r := prereleaseTool{}
	r.getPrereleases()
	r.sortReleasesByDate()
}

func TestOldReleaseIDs(t *testing.T) {
	tf.UnitTest(t)
	r := mock()
	r.getPrereleases()
	r.sortReleasesByDate()
	oldIDs := r.oldReleaseIDs()
	assert.Len(t, oldIDs, 1, "There should only be one ID")
	assert.Equal(t, oldIDs[0], int64(3), "Wrong ID to remove")
}

func TestOldReleaseIDsBoundsCheck(t *testing.T) {
	tf.UnitTest(t)
	r := prereleaseTool{}
	r.getPrereleases()
	r.sortReleasesByDate()
	oldIDs := r.oldReleaseIDs()
	assert.Len(t, oldIDs, 0, "There should only be no IDs")
}

func TestTrim(t *testing.T) {
	tf.UnitTest(t)
	ctx := context.Background()
	r := mock()
	r.getPrereleases()
	r.sortReleasesByDate()
	ok, err := r.trim(ctx, r.oldReleaseIDs())
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
	r.sortReleasesByDate()
	ok, err := r.trim(ctx, r.oldReleaseIDs())
	assert.False(t, ok, "This was a dry run, ok should be false")
	assert.NoError(t, err, "No error should exist")
}
