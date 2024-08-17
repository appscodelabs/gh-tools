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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/go-github/v61/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"gomodules.xyz/flags"
)

var dirStarReport = "/home/tamal/go/src/github.com/tamalsaha/star-report"

func NewCmdStarReport() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "star-report",
		Short:             "StarReport master and release-* repos",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			runStarReport()
		},
	}
	cmd.Flags().StringVar(&dirStarReport, "report-dir", dirStarReport, "Path to directory where star reports are stored")
	return cmd
}

func runStarReport() {
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
		opt := &github.RepositoryListByAuthenticatedUserOptions{
			Affiliation: "owner,organization_member",
			ListOptions: github.ListOptions{PerPage: 50},
		}
		repos, err := ListRepos(ctx, client, opt, false)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Found %d repositories", len(repos))
		for _, repo := range repos {
			if repo.GetOwner().GetType() == OwnerTypeUser {
				fmt.Printf("[ ] %s --- SKIPPED\n", repo.GetFullName())
				continue
			}

			if repo.GetPermissions()["admin"] {
				dir := filepath.Join(dirStarReport, repo.Owner.GetLogin(), repo.GetName())
				err = os.MkdirAll(dir, 0o755)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Printf("[x] %s >>> %s\n", repo.GetFullName(), dir)

				{
					o2 := &github.ListOptions{PerPage: 50}
					stargazers, err := ListStargazers(ctx, client, repo, o2)
					if err == nil {
						data, err := json.MarshalIndent(stargazers, "", "  ")
						if err == nil {
							err = os.WriteFile(filepath.Join(dir, "stargazers.json"), data, 0o644)
							if err != nil {
								log.Println(err)
							}
						}
					}
				}

				{
					o2 := &github.ListOptions{PerPage: 50}
					watchers, err := ListWatchers(ctx, client, repo, o2)
					if err == nil {
						data, err := json.MarshalIndent(watchers, "", "  ")
						if err == nil {
							err = os.WriteFile(filepath.Join(dir, "watchers.json"), data, 0o644)
							if err != nil {
								log.Println(err)
							}
						}
					}
				}
			}
		}
	}
}

func ListStargazers(ctx context.Context, client *github.Client, repo *github.Repository, opt *github.ListOptions) ([]*github.Stargazer, error) {
	var result []*github.Stargazer
	for {
		stargazer, resp, err := client.Activity.ListStargazers(ctx, repo.Owner.GetLogin(), repo.GetName(), opt)
		if err != nil {
			break
		}
		result = append(result, stargazer...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].GetUser().GetLogin()) < strings.ToLower(result[j].GetUser().GetLogin())
	})
	return result, nil
}

func ListWatchers(ctx context.Context, client *github.Client, repo *github.Repository, opt *github.ListOptions) ([]*github.User, error) {
	var result []*github.User
	for {
		stargazer, resp, err := client.Activity.ListWatchers(ctx, repo.Owner.GetLogin(), repo.GetName(), opt)
		if err != nil {
			break
		}
		result = append(result, stargazer...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].GetLogin()) < strings.ToLower(result[j].GetLogin())
	})
	return result, nil
}
