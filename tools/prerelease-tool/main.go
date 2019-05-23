package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type prereleaseTool struct {
	Releases    []*github.RepositoryRelease
	Limit       int
	Owner       string
	Repo        string
	client      *github.Client
	Prereleases []github.RepositoryRelease
	DryRun      bool
}

func main() {
	dryRun := flag.Bool("dry-run", false, "perform a dry run instead of executing create/delete/update actions")
	limit := flag.Int("limit", 7, "limit of prereleases to keep")
	owner := flag.String("owner", "filecoin-project", "github owner or organization")
	repo := flag.String("repo", "go-filecoin", "github project repository")
	token, ok := os.LookupEnv("GITHUB_TOKEN")
	if !ok {
		log.Fatal("Github token must be provided through GITHUB_TOKEN environment variable")
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
			log.Print(ctx.Err())
		}
	}()
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	flag.Parse()
	r := prereleaseTool{
		Limit:  *limit,
		Owner:  *owner,
		Repo:   *repo,
		client: client,
		DryRun: *dryRun,
	}
	if err := r.getReleases(ctx); err != nil {
		log.Fatalf("Could not find any releases: %+v", err)
	}
	r.getPrereleases()
	ok, err := r.deleteReleases(ctx, r.outdatedPreleaseIDs())
	if err != nil {
		log.Fatalf("Problem attempting to delete releases: %+v", err)
	}
	if !ok {
		log.Print("Remove --dry-run flag to apply changes")
	}
}

func (r *prereleaseTool) getReleases(ctx context.Context) error {
	releases, _, err := r.client.Repositories.ListReleases(ctx, r.Owner, r.Repo, nil)
	r.Releases = releases
	return err
}

func (r *prereleaseTool) getPrereleases() {
	switch {
	case len(r.Releases) > 0:
		for _, release := range r.Releases {
			if *release.Prerelease {
				r.Prereleases = append(r.Prereleases, *release)
			}
		}
		r.sortReleasesByDate()
	default:
		log.Print("no releases found")
	}

}

func (r *prereleaseTool) sortReleasesByDate() {
	sort.Slice(r.Prereleases, func(i, j int) bool {
		return r.Prereleases[i].CreatedAt.After(r.Prereleases[j].CreatedAt.Time)
	})
}

func (r *prereleaseTool) outdatedPreleaseIDs() []int64 {
	var idsToDelete []int64
	switch {
	case len(r.Prereleases) > r.Limit:
		trimmedPrereleaseList := r.Prereleases[r.Limit:]
		for _, release := range trimmedPrereleaseList {
			idsToDelete = append(idsToDelete, *release.ID)
		}
	default:
		log.Print("There are no outdated prereleases")
	}
	return idsToDelete
}

func (r *prereleaseTool) deleteReleases(ctx context.Context, ids []int64) (bool, error) {
	var ok bool
	var deleteCount int
	defer log.Printf("Deleted %d releases", deleteCount)
	if len(ids) > 0 {
		log.Print("Removing outdated Prereleases")
		for _, id := range ids {
			log.Printf("Prerelease ID %d selected for deletion", id)
			if !r.DryRun {
				ok = true
				log.Printf("Deleting Prerelease %d", id)
				resp, err := r.client.Repositories.DeleteRelease(ctx, r.Owner, r.Repo, id)
				if err != nil {
					return ok, err
				}
				if resp.StatusCode != 204 {
					return ok, fmt.Errorf("Unexpected HTTP status code. Expected: 204 Got: %d", resp.StatusCode)
				}
				deleteCount++
			}
		}
	}
	return ok, nil
}
