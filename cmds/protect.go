package cmds

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var (
	dryrun = false
)

func NewCmdProtect() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "protect",
		Short:             "Protect master and release-* repos",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			runProtect()
		},
	}
	return cmd
}

func runProtect() {
	token, found := os.LookupEnv("GH_TOOLS_TOKEN")
	if !found {
		log.Fatalln("GH_TOOLS_TOKEN env var is not set")
	}

	ctx := context.Background()

	// Create the http client.
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	// Get the current user
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("user: ", user.GetLogin())

	{
		opt := &github.ListOptions{PerPage: 50}
		orgs, err := ListOrgs(ctx, client, opt)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Found %d orgs", len(orgs))
		for _, org := range orgs {
			fmt.Println(">>> " + org.GetLogin())
		}
	}

	{
		opt := &github.RepositoryListOptions{
			Affiliation: "owner,organization_member",
			ListOptions: github.ListOptions{PerPage: 50},
		}
		repos, err := ListRepos(ctx, client, user.GetLogin(), opt)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Found %d repositories", len(repos))
		for _, repo := range repos {
			if repo.GetOwner().GetType() == "User" {
				continue
			}
			if repo.GetPermissions()["admin"] {
				ProtectRepo(ctx, client, repo)
			}
		}
	}
	/*
		{
			err := ProtectBranch(ctx, client, "yaxc", "test", "master")
			if err != nil {
				log.Fatal(err)
			}
		}
	*/
}

func ListOrgs(ctx context.Context, client *github.Client, opt *github.ListOptions) ([]*github.Organization, error) {
	var result []*github.Organization
	for {
		orgs, resp, err := client.Organizations.List(ctx, "", opt)
		if err != nil {
			return nil, err
		}
		result = append(result, orgs...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	sort.Slice(result, func(i, j int) bool { return result[i].GetLogin() < result[j].GetLogin() })
	return result, nil
}

func ListRepos(ctx context.Context, client *github.Client, user string, opt *github.RepositoryListOptions) ([]*github.Repository, error) {
	var result []*github.Repository
	for {
		repos, resp, err := client.Repositories.List(ctx, "", opt)
		if err != nil {
			return nil, err
		}
		result = append(result, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return result, nil
}

func ListBranches(ctx context.Context, client *github.Client, repo *github.Repository) ([]*github.Branch, error) {
	opt := &github.ListOptions{
		PerPage: 100,
	}

	var result []*github.Branch
	for {
		branch, resp, err := client.Repositories.ListBranches(ctx, repo.Owner.GetLogin(), repo.GetName(), opt)
		if err != nil {
			return nil, err
		}
		result = append(result, branch...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return result, nil
}

func ProtectRepo(ctx context.Context, client *github.Client, repo *github.Repository) error {
	branches, err := ListBranches(ctx, client, repo)
	if err != nil {
		return err
	}

	for _, branch := range branches {
		if branch.GetName() == "master" || strings.HasPrefix(branch.GetName(), "release-") {
			if err := ProtectBranch(ctx, client, repo.Owner.GetLogin(), repo.GetName(), branch.GetName()); err != nil {
				return err
			}
		}
	}

	return nil
}

func ProtectBranch(ctx context.Context, client *github.Client, owner, repo, branch string) error {
	fmt.Printf("[UPDATE] %s/%s:%s will be changed to protected\n", owner, repo, branch)
	if dryrun {
		// return early
		return nil
	}

	// set the branch to be protected
	p := &github.ProtectionRequest{
		RequiredStatusChecks: &github.RequiredStatusChecks{
			Strict:   false,
			Contexts: []string{},
		},
		RequiredPullRequestReviews: &github.PullRequestReviewsEnforcementRequest{
			DismissStaleReviews:     true,
			RequireCodeOwnerReviews: true,
		},
		// EnforceAdmins: true,
		Restrictions: &github.BranchRestrictionsRequest{
			Users: []string{},
			Teams: []string{},
		},
	}
	_, _, err := client.Repositories.UpdateBranchProtection(ctx, owner, repo, branch, p)
	return err
}
