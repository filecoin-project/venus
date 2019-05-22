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
	Data        []*github.RepositoryRelease
	Limit       int
	Owner       string
	Repo        string
	client      *github.Client
	Prereleases []github.RepositoryRelease
	DryRun      bool
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	token, ok := os.LookupEnv("GITHUB_TOKEN")
	if !ok {
		log.Fatal("Github token must be provided through GITHUB_TOKEN environment variable")
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	dryRun := flag.Bool("dry-run", false, "perform a dry run instead of executing create/delete/update actions")
	limit := flag.Int("limit", 7, "limit of prereleases to keep")
	owner := flag.String("owner", "filecoin-project", "github owner or organization")
	repo := flag.String("repo", "go-filecoin", "github project repository")
	flag.Parse()
	r := prereleaseTool{
		Limit:  *limit,
		Owner:  *owner,
		Repo:   *repo,
		client: client,
		DryRun: *dryRun,
	}
	if err := r.get(ctx); err != nil {
		log.Fatalf("Could not find any releases: %+v", err)
	}
	r.getPrereleases()
	r.sortReleasesByDate()
	ok, err := r.trim(ctx, r.oldReleaseIDs())
	if err != nil {
		log.Fatalf("Problem attempting to delete releases: %+v", err)
	}
	if !ok {
		log.Print("Remove --dry-run flag to apply changes")
	}
}

func (r *prereleaseTool) get(ctx context.Context) error {
	releases, _, err := r.client.Repositories.ListReleases(ctx, r.Owner, r.Repo, nil)
	r.Data = releases
	return err
}

func (r *prereleaseTool) getPrereleases() {
	switch {
	case len(r.Data) > 0:
		for _, release := range r.Data {
			if *release.Prerelease {
				r.Prereleases = append(r.Prereleases, *release)
			}
		}
	default:
		log.Print("no releases found")
	}

}

func (r *prereleaseTool) sortReleasesByDate() {
	sort.Slice(r.Prereleases, func(i, j int) bool {
		first := r.Prereleases[i].CreatedAt
		second := r.Prereleases[j].CreatedAt
		switch {
		case first.Unix() < second.Unix():
			return false
		default:
			return true
		}
	})
}

func (r *prereleaseTool) oldReleaseIDs() []int64 {
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

func (r *prereleaseTool) trim(ctx context.Context, ids []int64) (bool, error) {
	var ok bool
	switch {
	case len(ids) > 0:
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
			}
		}
	default:
		log.Print("No preleases to delete")
	}
	return ok, nil
}
