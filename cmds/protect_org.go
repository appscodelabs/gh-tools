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
	"time"

	"github.com/google/go-github/v81/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"gomodules.xyz/flags"
	"gomodules.xyz/sets"
)

func NewCmdProtectOrg() *cobra.Command {
	var (
		org         string
		orgSkipList []string
	)

	cmd := &cobra.Command{
		Use:               "protect-org",
		Short:             "Protect master and release-* branches for all repos in an organization",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			runProtectOrg(org, fork, orgSkipList)
		},
	}
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name (required)")
	cmd.Flags().BoolVar(&fork, "fork", false, "If true, include forked repos")
	cmd.Flags().StringSliceVar(&orgSkipList, "skip", nil, "Skip repositories (repo names without org prefix)")
	_ = cmd.MarkFlagRequired("org")
	return cmd
}

func runProtectOrg(org string, includeForks bool, skipList []string) {
	if org == "" {
		log.Fatal("--org flag is required")
	}

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

	// Get org info to check plan
	orgInfo, _, err := client.Organizations.Get(ctx, org)
	if err != nil {
		log.Fatal(err)
	}
	isFreeOrg := orgInfo.GetPlan().GetName() == "free"
	fmt.Printf(">>> Processing org: %s (plan: %s)\n", org, orgInfo.GetPlan().GetName())

	// Create reviewers team if missing
	if org == "appscode" {
		_, err = CreateTeamIfMissing(ctx, client, org, teamBEReviewers)
		if err != nil {
			log.Fatal(err)
		}
		_, err = CreateTeamIfMissing(ctx, client, org, teamFEReviewers)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		_, err = CreateTeamIfMissing(ctx, client, org, teamReviewers)
		if err != nil {
			log.Fatal(err)
		}
	}

	// List all repos in the org
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 50},
	}
	repos, err := ListOrgRepos(ctx, client, org, opt, includeForks)
	if err != nil {
		log.Fatal(err)
	}

	skipRepos := sets.NewString(skipList...)
	log.Printf("Found %d repositories in org %s", len(repos), org)

	for _, repo := range repos {
		if !repo.GetPermissions()["admin"] {
			log.Printf("Skipping %s (no admin permission)", repo.GetFullName())
			continue
		}

		// For appscode org, repos are added to team manually
		if org != "appscode" {
			err = TeamMaintainsRepo(ctx, client, org, teamReviewers, repo.GetName())
			if err != nil {
				log.Fatalln(err)
			}
		}

		// Skip private repos on free orgs (no branch protection available)
		if isFreeOrg && repo.GetPrivate() {
			log.Printf("Skipping %s (private repo on free org)", repo.GetFullName())
			continue
		}

		if skipRepos.Has(repo.GetName()) {
			log.Printf("Skipping %s (in skip list)", repo.GetFullName())
			continue
		}

		err = ProtectRepo(ctx, client, repo)
		if err != nil {
			log.Fatalln(err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	log.Printf("Finished protecting repos in org %s", org)
}
