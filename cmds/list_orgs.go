package cmds

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

func NewCmdListOrgs() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list-orgs",
		Short:             "List orgs",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			runListOrgs()
		},
	}
	return cmd
}

func runListOrgs() {
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
		opt := &github.ListOptions{PerPage: 10}
		orgs, err := ListOrgs(ctx, client, opt)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println()
		log.Printf("Found %d orgs", len(orgs))
		fmt.Println()
		for _, org := range orgs {
			fmt.Println(org.GetLogin())
		}
	}
}