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

// nolint:goconst
package cmds

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/go-github/v81/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"gomodules.xyz/flags"
)

var (
	enableDependabot    = true
	enableSecurityFixes = true
)

func NewCmdDependabot() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "dependabot",
		Short:             "Enable/disable Dependabot alerts",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			addDependabot()
		},
	}
	cmd.Flags().IntVar(&shards, "shards", shards, "Total number of shards")
	cmd.Flags().IntVar(&shardIndex, "shard-index", shardIndex, "Shard Index to be processed")
	cmd.Flags().BoolVar(&fork, "fork", fork, "If true, return forked repos")
	cmd.Flags().BoolVar(&enableDependabot, "enable", enableDependabot, "If true, activates Dependabot alerts")
	cmd.Flags().BoolVar(&enableSecurityFixes, "autofix", enableSecurityFixes, "If true, enables automatic security fixes")
	return cmd
}

func addDependabot() {
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

		orgs = ShardOrgs(orgs, shardIndex, shards)
		log.Printf("Found %d orgs", len(orgs))

		for _, org := range orgs {
			fmt.Println(">>> " + org.GetLogin())
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
		log.Printf("Found %d repositories", len(repos))
		for _, repo := range repos {
			if repo.GetOwner().GetType() == OwnerTypeUser {
				continue
			}
			//if repo.GetPrivate() {
			//	continue
			//}
			if repo.GetPermissions()["admin"] {
				err = processDependabot(ctx, client, repo)
				if err != nil {
					log.Fatalln(err)
				}
			}
		}
	}
}

func processDependabot(ctx context.Context, client *github.Client, repo *github.Repository) error {
	fmt.Println("[___]>", repo.Owner.GetLogin()+"/"+repo.GetName())

	if enableDependabot {
		if _, err := client.Repositories.EnableVulnerabilityAlerts(ctx, repo.Owner.GetLogin(), repo.GetName()); err != nil {
			return err
		}
		if enableSecurityFixes {
			if _, err := client.Repositories.EnableAutomatedSecurityFixes(ctx, repo.Owner.GetLogin(), repo.GetName()); err != nil {
				return err
			}
		}
	} else {
		if _, err := client.Repositories.DisableVulnerabilityAlerts(ctx, repo.Owner.GetLogin(), repo.GetName()); err != nil {
			return err
		}
		if _, err := client.Repositories.DisableAutomatedSecurityFixes(ctx, repo.Owner.GetLogin(), repo.GetName()); err != nil {
			return err
		}
	}
	return nil
}
