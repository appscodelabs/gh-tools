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

	"github.com/google/go-github/v84/github"
	"github.com/spf13/cobra"
	"gomodules.xyz/flags"
)

func NewCmdProtectRepo() *cobra.Command {
	var (
		owner string
		repo  string
	)
	cmd := &cobra.Command{
		Use:               "protect-repo",
		Short:             "Protect master/main, release-*, kubernetes-*, and ac-* branches in a repository",
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
	ctx := context.Background()
	client := newGitHubClient(ctx)

	r, err := GetRepo(ctx, client, owner, repo)
	if err != nil {
		log.Fatalln(err)
	}
	if r == nil {
		log.Printf("repository not found: %s/%s", owner, repo)
		return
	}

	supported, reason, err := repoSupportsProtection(ctx, client, r)
	if err != nil {
		log.Fatalln(err)
	}
	if !supported {
		log.Printf("Skipping %s (%s)", r.GetFullName(), reason)
		return
	}

	err = ProtectRepo(ctx, client, r)
	if err != nil {
		log.Fatalln(err)
	}
}

func GetRepo(ctx context.Context, client *github.Client, owner, repo string) (*github.Repository, error) {
	r, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		if e, ok := err.(*github.ErrorResponse); ok && e.Response.StatusCode == http.StatusNotFound {
			log.Println(err)
			return nil, nil
		}
		return nil, err
	}
	return r, nil
}
