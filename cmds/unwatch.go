package cmds

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/google/go-github/v25/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var (
	orgsToWatchRepos []string
)

func NewCmdStopWatch() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "stop-watching",
		Short:             "Stop watching repos of a org",
		DisableAutoGenTag: true,
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

				client.Activity.DeleteRepositorySubscription(ctx, repo.Owner.GetLogin(), repo.GetName())
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
