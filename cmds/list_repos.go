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

	"github.com/google/go-github/v50/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"gomodules.xyz/sets"
)

func NewCmdListRepos() *cobra.Command {
	var orgs []string
	var orgOwned bool
	cmd := &cobra.Command{
		Use:               "list-repos",
		Short:             "List repos for repo-refresher scripts",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			printRepoList(sets.NewString(orgs...), orgOwned, fork)
		},
	}
	cmd.Flags().StringSliceVar(&orgs, "orgs", orgs, "Orgs for which repo list will be printed")
	cmd.Flags().BoolVar(&fork, "fork", fork, "If true, return forked repos")
	cmd.Flags().BoolVar(&orgOwned, "org-owned", orgOwned, "If true, return org owned repos")
	return cmd
}

func printRepoList(orgs sets.String, orgOwned, fork bool) {
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

	var listing []string
	if orgOwned {
		opt := &github.RepositoryListByOrgOptions{
			Type:        "public",
			ListOptions: github.ListOptions{PerPage: 50},
		}
		for _, org := range orgs.List() {
			repos, err := ListOrgRepos(ctx, client, org, opt, fork)
			if err != nil {
				log.Fatal(err)
			}
			for _, repo := range repos {
				listing = append(listing, fmt.Sprintf("github.com/%s/%s", repo.GetOwner().GetLogin(), repo.GetName()))
			}
		}
	} else {
		opt := &github.RepositoryListOptions{
			Affiliation: "owner,organization_member",
			ListOptions: github.ListOptions{PerPage: 50},
		}
		repos, err := ListRepos(ctx, client, opt, fork)
		if err != nil {
			log.Fatal(err)
		}
		listing = make([]string, 0, len(repos))
		for _, repo := range repos {
			if repo.GetOwner().GetType() == OwnerTypeUser {
				continue // don't protect personal repos
			}
			if repo.GetPermissions()["admin"] && (orgs.Len() == 0 || orgs.Has(repo.GetOwner().GetLogin())) {
				listing = append(listing, fmt.Sprintf("github.com/%s/%s", repo.GetOwner().GetLogin(), repo.GetName()))
			}
		}
	}

	sort.Strings(listing)
	for _, entry := range listing {
		fmt.Println(entry)
	}
}
