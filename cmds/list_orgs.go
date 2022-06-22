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

	"github.com/google/go-github/v45/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"gomodules.xyz/flags"
)

func NewCmdListOrgs() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list-orgs",
		Short:             "List orgs",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			runListOrgs()
		},
	}
	cmd.Flags().IntVar(&shards, "shards", shards, "Total number of shards")
	cmd.Flags().IntVar(&shardIndex, "shard-index", shardIndex, "Shard Index to be processed")
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
		opt := &github.ListOptions{PerPage: 50}
		orgs, err := ListOrgs(ctx, client, opt)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println()

		orgs = ShardOrgs(orgs, shardIndex, shards)
		log.Printf("Found %d orgs", len(orgs))
		fmt.Println()

		for _, org := range orgs {
			fmt.Println(org.GetLogin())
		}
	}
}
