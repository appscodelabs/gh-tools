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
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/go-github/v81/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"gomodules.xyz/flags"
)

func NewCmdProtectRepo() *cobra.Command {
	var (
		owner string
		repo  string
	)
	cmd := &cobra.Command{
		Use:               "protect-repo",
		Short:             "Protect master and release-* repos",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			runProtectRepo(owner, repo)
		},
	}
	cmd.Flags().StringVar(&owner, "owner", owner, "GitHub user or org name")
	cmd.Flags().StringVar(&repo, "repo", repo, "GitHub repository name")
	return cmd
}

func runProtectRepo(owner, repo string) {
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

	r, err := GetRepo(ctx, client, owner, repo)
	if err != nil {
		log.Fatalln(err)
	}

	err = ProtectRepo(ctx, client, r)
	if err != nil {
		log.Fatalln(err)
	}
}

func GetRepo(ctx context.Context, client *github.Client, owner, repo string) (*github.Repository, error) {
	for {
		repo, _, err := client.Repositories.Get(ctx, owner, repo)
		switch e := err.(type) {
		case *github.RateLimitError:
			time.Sleep(time.Until(e.Rate.Reset.Add(skew)))
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

		return repo, nil
	}
}
