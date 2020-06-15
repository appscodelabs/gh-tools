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
	"os"
	"sort"
	"strings"

	"github.com/google/go-github/v32/github"
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
	cmd.Flags().BoolVar(&dryrun, "dryrun", dryrun, "If set to true, will not apply changes.")
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
			if repo.GetOwner().GetType() == OwnerTypeUser {
				continue
			}
			if repo.GetPrivate() {
				continue
			}
			if repo.GetArchived() {
				continue
			}
			//if repo.GetFork() {
			//	continue
			//}
			if repo.GetPermissions()["admin"] {
				err = ProtectRepo(ctx, client, repo)
				if err != nil {
					log.Fatalln(err)
				}
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
			break
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
			break
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
	opt := &github.BranchListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var result []*github.Branch
	for {
		branch, resp, err := client.Repositories.ListBranches(ctx, repo.Owner.GetLogin(), repo.GetName(), opt)
		if err != nil {
			break
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
			Strict: true,
			Contexts: []string{
				"Build",
				"DCO",
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
		},
	}
	if repo == "installer" {
		p.RequiredStatusChecks.Contexts = append(
			p.RequiredStatusChecks.Contexts,
			"Kubernetes (v1.12.10)",
			"Kubernetes (v1.13.12)",
			"Kubernetes (v1.14.10)",
			"Kubernetes (v1.15.7)",
			"Kubernetes (v1.16.9)",
			"Kubernetes (v1.17.2)",
			"Kubernetes (v1.18.2)",
		)
	}
	if branch == "master" {
		p.Restrictions.Apps = []string{"kodiakhq"}
	}
	_, _, err := client.Repositories.UpdateBranchProtection(ctx, owner, repo, branch, p)
	return err
}
