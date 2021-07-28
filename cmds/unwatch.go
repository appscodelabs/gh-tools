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

	"github.com/google/go-github/v35/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"gomodules.xyz/flags"
)

var (
	orgsToWatchRepos []string
)

func NewCmdStopWatch() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "stop-watching",
		Short:             "Stop watching repos of a org",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			runStopWatch()
		},
	}
	cmd.Flags().StringSliceVar(&orgsToWatchRepos, "orgs", nil, "")
	return cmd
}

func runStopWatch() {
	token, found := os.LookupEnv("GH_TOOLS_TOKEN")
	if !found {
		log.Fatalln("GH_TOOLS_TOKEN env var is not set")
	}

	if len(orgsToWatchRepos) == 0 {
		os.Exit(0)
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
		repos, err := ListWatchedRepos(ctx, client, opt)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Found %d orgs", len(repos))
		for _, repo := range repos {
			if in(orgsToWatchRepos, repo.GetOwner().GetLogin()) {
				fmt.Printf("[UPDATE] Stopping to watch %s/%s\n", repo.Owner.GetLogin(), repo.GetName())

				_, err := client.Activity.DeleteRepositorySubscription(ctx, repo.Owner.GetLogin(), repo.GetName())
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}

func ListWatchedRepos(ctx context.Context, client *github.Client, opt *github.ListOptions) ([]*github.Repository, error) {
	var result []*github.Repository
	for {
		orgs, resp, err := client.Activity.ListWatched(ctx, "", opt)
		if err != nil {
			return nil, err
		}
		result = append(result, orgs...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	sort.Slice(result, func(i, j int) bool { return result[i].GetFullName() < result[j].GetFullName() })
	return result, nil
}

func in(a []string, s string) bool {
	for _, b := range a {
		if b == s {
			return true
		}
	}
	return false
}
