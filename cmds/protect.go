/*
Copyright AppsCode Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmds

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v69/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"gomodules.xyz/flags"
	"gomodules.xyz/sets"
)

const (
	skew            = 10 * time.Second
	teamReviewers   = "reviewers"
	teamFEReviewers = "fe-reviewers" // frontend
	teamBEReviewers = "be-reviewers" // backend
)

var (
	dryrun     = false
	freeOrgs   = map[string]bool{}
	shardIndex = -1
	shards     = -1
	fork       bool
	skipList   []string
)

func NewCmdProtect() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "protect",
		Short:             "Protect master and release-* repos",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			runProtect()
		},
	}
	cmd.Flags().BoolVar(&dryrun, "dryrun", dryrun, "If set to true, will not apply changes.")
	cmd.Flags().IntVar(&shards, "shards", shards, "Total number of shards")
	cmd.Flags().IntVar(&shardIndex, "shard-index", shardIndex, "Shard Index to be processed")
	cmd.Flags().BoolVar(&fork, "fork", fork, "If true, return forked repos")
	cmd.Flags().StringSliceVar(&skipList, "skip", skipList, "Skip owner/repository")
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

	//{
	//	p, _, err := client.Repositories.GetBranchProtection(ctx, "stashed", "apimachinery", "master")
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//	data, err := json.MarshalIndent(p, "", "  ")
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//	fmt.Println(string(data))
	//	os.Exit(1)
	//}

	{
		opt := &github.ListOptions{PerPage: 50}
		orgs, err := ListOrgs(ctx, client, opt)
		if err != nil {
			log.Fatal(err)
		}

		orgs = ShardOrgs(orgs, shardIndex, shards)
		log.Printf("Found %d orgs", len(orgs))

		for _, org := range orgs {
			fmt.Println(">>> " + org.GetLogin())
			// list orgs api does not return plan info
			r, _, err := client.Organizations.Get(ctx, org.GetLogin())
			if err != nil {
				log.Fatal(err)
			}
			freeOrgs[r.GetLogin()] = r.GetPlan().GetName() == "free"

			if r.GetLogin() == "appscode" {
				_, err = CreateTeamIfMissing(ctx, client, r.GetLogin(), teamBEReviewers)
				if err != nil {
					log.Fatal(err)
				}
				_, err = CreateTeamIfMissing(ctx, client, r.GetLogin(), teamFEReviewers)
				if err != nil {
					log.Fatal(err)
				}
			} else {
				_, err = CreateTeamIfMissing(ctx, client, r.GetLogin(), teamReviewers)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}

	{
		opt := &github.RepositoryListByAuthenticatedUserOptions{
			Affiliation: "owner,organization_member",
			ListOptions: github.ListOptions{PerPage: 50},
		}
		repos, err := ListRepos(ctx, client, opt, fork)
		if err != nil {
			log.Fatal(err)
		}
		skipRepos := sets.NewString(skipList...)
		log.Printf("Found %d repositories", len(repos))
		for _, repo := range repos {
			if repo.GetOwner().GetType() == OwnerTypeUser {
				continue // don't protect personal repos
			}
			if repo.GetPermissions()["admin"] {
				// for appscode org, add repos by hand to team
				if repo.GetOwner().GetLogin() != "appscode" {
					err = TeamMaintainsRepo(ctx, client, repo.GetOwner().GetLogin(), teamReviewers, repo.GetName())
					if err != nil {
						log.Fatalln(err)
					}
				}

				if freeOrgs[repo.GetOwner().GetLogin()] && repo.GetPrivate() {
					continue
				}
				if skipRepos.Has(repo.GetFullName()) {
					continue
				}

				err = ProtectRepo(ctx, client, repo)
				if err != nil {
					log.Fatalln(err)
				}
				time.Sleep(10 * time.Millisecond)
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
		switch e := err.(type) {
		case *github.RateLimitError:
			time.Sleep(time.Until(e.Rate.Reset.Time.Add(skew)))
			continue
		case *github.AbuseRateLimitError:
			time.Sleep(e.GetRetryAfter())
			continue
		case *github.ErrorResponse:
			if e.Response.StatusCode == http.StatusNotFound {
				log.Println(err)
				break
			} else {
				return nil, err
			}
		default:
			if e != nil {
				return nil, err
			}
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

func ShardOrgs(in []*github.Organization, shardIndex, shards int) []*github.Organization {
	if shardIndex < 0 || shards < 1 {
		return in
	}
	sort.Slice(in, func(i, j int) bool {
		return in[i].GetLogin() < in[j].GetLogin()
	})
	itemsPerShard := int(math.Ceil(float64(len(in)) / float64(shards)))
	start := shardIndex * itemsPerShard
	end := min(start+itemsPerShard, len(in))
	return in[start:end]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ListRepos(ctx context.Context, client *github.Client, opt *github.RepositoryListByAuthenticatedUserOptions, fork bool) ([]*github.Repository, error) {
	var result []*github.Repository
	for {
		repos, resp, err := client.Repositories.ListByAuthenticatedUser(ctx, opt)
		switch e := err.(type) {
		case *github.RateLimitError:
			time.Sleep(time.Until(e.Rate.Reset.Time.Add(skew)))
			continue
		case *github.AbuseRateLimitError:
			time.Sleep(e.GetRetryAfter())
			continue
		case *github.ErrorResponse:
			if e.Response.StatusCode == http.StatusNotFound {
				log.Println(err)
				break
			} else {
				return nil, err
			}
		default:
			if e != nil {
				return nil, err
			}
		}

		for idx := range repos {
			if repos[idx].GetArchived() {
				continue
			}
			if repos[idx].GetFork() && !fork {
				continue
			}
			result = append(result, repos[idx])
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return result, nil
}

func ListOrgRepos(ctx context.Context, client *github.Client, org string, opt *github.RepositoryListByOrgOptions, fork bool) ([]*github.Repository, error) {
	var result []*github.Repository
	for {
		repos, resp, err := client.Repositories.ListByOrg(ctx, org, opt)
		switch e := err.(type) {
		case *github.RateLimitError:
			time.Sleep(time.Until(e.Rate.Reset.Time.Add(skew)))
			continue
		case *github.AbuseRateLimitError:
			time.Sleep(e.GetRetryAfter())
			continue
		case *github.ErrorResponse:
			if e.Response.StatusCode == http.StatusNotFound {
				log.Println(err)
				break
			} else {
				return nil, err
			}
		default:
			if e != nil {
				return nil, err
			}
		}

		for idx := range repos {
			if repos[idx].GetArchived() {
				continue
			}
			if repos[idx].GetFork() && !fork {
				continue
			}
			result = append(result, repos[idx])
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return result, nil
}

func ListBranches(ctx context.Context, client *github.Client, repo *github.Repository) ([]*github.Branch, error) {
	opt := &github.BranchListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var result []*github.Branch
	for {
		branch, resp, err := client.Repositories.ListBranches(ctx, repo.Owner.GetLogin(), repo.GetName(), opt)
		switch e := err.(type) {
		case *github.RateLimitError:
			time.Sleep(time.Until(e.Rate.Reset.Time.Add(skew)))
			continue
		case *github.AbuseRateLimitError:
			time.Sleep(e.GetRetryAfter())
			continue
		case *github.ErrorResponse:
			if e.Response.StatusCode == http.StatusNotFound {
				log.Println(err)
				break
			} else {
				return nil, err
			}
		default:
			if e != nil {
				return nil, err
			}
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
		if branch.GetName() == "master" ||
			strings.HasPrefix(branch.GetName(), "release-") ||
			strings.HasPrefix(branch.GetName(), "kubernetes-") ||
			strings.HasPrefix(branch.GetName(), "ac-") {
			if err := ProtectBranch(ctx, client, repo.Owner.GetLogin(), repo.GetName(), branch.GetName(), repo.GetPrivate()); err != nil {
				switch e := err.(type) {
				case *github.RateLimitError:
					time.Sleep(time.Until(e.Rate.Reset.Time.Add(skew)))
					continue
				case *github.AbuseRateLimitError:
					time.Sleep(e.GetRetryAfter())
					continue
				case *github.ErrorResponse:
					log.Println("error", err)
				}
				return nil // ignore error
			}
		}
	}
	return nil
}

var requiredStatusChecks = map[string][]*github.RequiredStatusCheck{
	"kubeform/gen-repo-refresher": {{Context: "DCO"}, {Context: "license/cla"}},
	"kubeguard/guard":             {{Context: "DCO"}, {Context: "Build"}},
}

func ProtectBranch(ctx context.Context, client *github.Client, owner, repo, branch string, private bool) error {
	fmt.Printf("[UPDATE] %s/%s:%s will be changed to protected\n", owner, repo, branch)
	if dryrun {
		// return early
		return nil
	}

	// set the branch to be protected
	p := &github.ProtectionRequest{
		RequiredStatusChecks: &github.RequiredStatusChecks{
			Strict: true,
			Checks: &[]*github.RequiredStatusCheck{
				{Context: "Build"},
				{Context: "DCO"},
			},
		},
		RequiredPullRequestReviews: &github.PullRequestReviewsEnforcementRequest{
			DismissStaleReviews: true,
			DismissalRestrictionsRequest: &github.DismissalRestrictionsRequest{
				Users: nil,
				Teams: nil,
			},
			RequireCodeOwnerReviews:      false,
			RequiredApprovingReviewCount: 1,
		},
		// EnforceAdmins: true,
		Restrictions: &github.BranchRestrictionsRequest{
			Users: make([]string, 1),
			Teams: make([]string, 1),
			// Apps:  []string{"kodiakhq"},
		},
	}
	if owner == "appscode-cloud" || owner == "kubedb" || owner == "kubestash" {
		p.Restrictions.Apps = []string{"kodiak-appscode"}
	} else {
		p.Restrictions.Apps = []string{"kodiakhq"}
	}

	if !private {
		checks := append(
			*p.RequiredStatusChecks.Checks,
			&github.RequiredStatusCheck{Context: "license/cla"},
		)
		p.RequiredStatusChecks.Checks = &checks
	}

	if repo == "installer" ||
		(owner == "stashed" && repo == "catalog") {
		checks := append(
			*p.RequiredStatusChecks.Checks,
			&github.RequiredStatusCheck{Context: "Kubernetes (v1.29.14)"},
			&github.RequiredStatusCheck{Context: "Kubernetes (v1.30.10)"},
			&github.RequiredStatusCheck{Context: "Kubernetes (v1.31.6)"},
			&github.RequiredStatusCheck{Context: "Kubernetes (v1.32.2)"},
		)
		p.RequiredStatusChecks.Checks = &checks
	}
	if repo == "ui-wizards" {
		checks := append(
			*p.RequiredStatusChecks.Checks,
			&github.RequiredStatusCheck{Context: "Kubernetes (v1.29.14)"},
			&github.RequiredStatusCheck{Context: "Kubernetes (v1.30.10)"},
			&github.RequiredStatusCheck{Context: "Kubernetes (v1.31.6)"},
			&github.RequiredStatusCheck{Context: "Kubernetes (v1.32.2)"},
		)
		p.RequiredStatusChecks.Checks = &checks
	}
	if owner == "voyagermesh" {
		checks := append(
			*p.RequiredStatusChecks.Checks,
			&github.RequiredStatusCheck{Context: "Kubernetes (v1.29.14)"},
			&github.RequiredStatusCheck{Context: "Kubernetes (v1.30.10)"},
			&github.RequiredStatusCheck{Context: "Kubernetes (v1.31.6)"},
			&github.RequiredStatusCheck{Context: "Kubernetes (v1.32.2)"},
		)
		p.RequiredStatusChecks.Checks = &checks
	}

	if strings.EqualFold(repo, "CHANGELOG") {
		// Avoid dismissing stale reviews, since delay in kodiak auto approval can fail release process.
		p.RequiredPullRequestReviews.DismissStaleReviews = false
		p.RequiredStatusChecks.Checks = &[]*github.RequiredStatusCheck{
			{Context: "DCO"},
		}
	}
	//if branch == "master" {
	//	p.Restrictions.Apps = []string{"kodiakhq"}
	//}

	if predefinedChecks, ok := requiredStatusChecks[fmt.Sprintf("%s/%s", owner, repo)]; ok {
		p.RequiredStatusChecks.Checks = &predefinedChecks
	}

	_, _, err := client.Repositories.UpdateBranchProtection(ctx, owner, repo, branch, p)
	return err
}

func TeamMaintainsRepo(ctx context.Context, client *github.Client, org, team, repo string) error {
	for {
		_, err := client.Teams.AddTeamRepoBySlug(ctx, org, team, org, repo, &github.TeamAddTeamRepoOptions{
			Permission: "admin",
		})
		switch e := err.(type) {
		case *github.RateLimitError:
			time.Sleep(time.Until(e.Rate.Reset.Time.Add(skew)))
			continue
		case *github.AbuseRateLimitError:
			time.Sleep(e.GetRetryAfter())
			continue
		case *github.ErrorResponse:
			if e.Response.StatusCode == http.StatusNotFound {
				log.Println(err)
				break
			} else {
				return err
			}
		default:
			if e != nil {
				return err
			}
		}
		return nil
	}
}

func CreateTeamIfMissing(ctx context.Context, client *github.Client, org, team string) (int64, error) {
GET_TEAM:
	for {
		t, _, err := client.Teams.GetTeamBySlug(ctx, org, team)
		switch e := err.(type) {
		case *github.RateLimitError:
			time.Sleep(time.Until(e.Rate.Reset.Time.Add(skew)))
			continue
		case *github.AbuseRateLimitError:
			time.Sleep(e.GetRetryAfter())
			continue
		case *github.ErrorResponse:
			if e.Response.StatusCode == http.StatusNotFound {
				log.Println(err)
				break GET_TEAM
			} else {
				return 0, err
			}
		default:
			if e != nil {
				return 0, err
			}
		}
		return t.GetID(), nil // team exists
	}

	privacy := "closed"
	for {
		t, _, err := client.Teams.CreateTeam(ctx, org, github.NewTeam{
			Name: team,
			Maintainers: []string{
				"tamalsaha",
				"1gtm",
			},
			Privacy: &privacy,
		})
		switch e := err.(type) {
		case *github.RateLimitError:
			time.Sleep(time.Until(e.Rate.Reset.Time.Add(skew)))
			continue
		case *github.AbuseRateLimitError:
			time.Sleep(e.GetRetryAfter())
			continue
		case *github.ErrorResponse:
			return 0, err
		default:
			if e != nil {
				return 0, err
			}
		}
		return t.GetID(), nil
	}
}
