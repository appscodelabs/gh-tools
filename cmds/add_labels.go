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

	"github.com/appscode/go/types"
	"github.com/google/go-github/v32/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

func NewCmdAddLabels() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "add-labels",
		Short:             "Add labels",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			addLabels()
		},
	}
	return cmd
}

func addLabels() {
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
			if repo.GetPrivate() {
				continue
			}
			if repo.GetArchived() {
				continue
			}
			if repo.GetPermissions()["admin"] {
				err = AddLabelToRepo(ctx, client, repo)
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

func ListLabels(ctx context.Context, client *github.Client, repo *github.Repository) ([]*github.Label, error) {
	opt := &github.ListOptions{
		PerPage: 100,
	}

	var result []*github.Label
	for {
		labels, resp, err := client.Issues.ListLabels(ctx, repo.Owner.GetLogin(), repo.GetName(), opt)
		if err != nil {
			break
		}
		result = append(result, labels...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return result, nil
}

func LabelExists(labels []*github.Label, name string) (*github.Label, bool) {
	for _, label := range labels {
		if label.GetName() == name {
			return label, true
		}
	}
	return nil, false
}

func AddLabelToRepo(ctx context.Context, client *github.Client, repo *github.Repository) error {
	fmt.Println("[___]>", repo.Owner.GetLogin()+"/"+repo.GetName())
	labels, err := ListLabels(ctx, client, repo)
	if err != nil {
		return err
	}

	/*
		automerge: Kodiak will auto merge PRs that have this label : #fef2c0
	*/
	if label, ok := LabelExists(labels, "automerge"); ok {
		if label.GetColor() != "fef2c0" {
			_, _, err := client.Issues.EditLabel(ctx, repo.Owner.GetLogin(), repo.GetName(), "automerge", &github.Label{
				Name:        types.StringP("automerge"),
				Color:       types.StringP("fef2c0"),
				Description: types.StringP("Kodiak will auto merge PRs that have this label"),
			})
			if err != nil {
				return err
			}
		}
	} else {
		_, _, err := client.Issues.CreateLabel(ctx, repo.Owner.GetLogin(), repo.GetName(), &github.Label{
			Name:        types.StringP("automerge"),
			Color:       types.StringP("fef2c0"),
			Description: types.StringP("Kodiak will auto merge PRs that have this label"),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
