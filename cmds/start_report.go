package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var (
	dirStarReport = "/home/tamal/go/src/github.com/tamalsaha/star-report"
)

func NewCmdStarReport() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "star-report",
		Short:             "StarReport master and release-* repos",
		DisableAutoGenTag: true,
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
			if repo.GetPermissions()["admin"] {
				dir := filepath.Join(dirStarReport, repo.Owner.GetLogin(), repo.GetName())
				err = os.MkdirAll(dir, 0755)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Printf("[x] %s >>> %s\n", repo.GetFullName(), dir)

				{
					o2 := &github.ListOptions{PerPage: 50}
					stargazers, err := ListStargazers(ctx, client, repo, o2)
					if err != nil {
						log.Fatal(err)
					}
					data, err := json.MarshalIndent(stargazers, "", "  ")
					if err != nil {
						log.Fatal(err)
					}
					err = ioutil.WriteFile(filepath.Join(dir, "stargazers.json"), data, 0644)
					if err != nil {
						log.Fatal(err)
					}
				}

				{
					o2 := &github.ListOptions{PerPage: 50}
					watchers, err := ListWatchers(ctx, client, repo, o2)
					if err != nil {
						log.Fatal(err)
					}
					data, err := json.MarshalIndent(watchers, "", "  ")
					if err != nil {
						log.Fatal(err)
					}
					err = ioutil.WriteFile(filepath.Join(dir, "watchers.json"), data, 0644)
					if err != nil {
						log.Fatal(err)
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
