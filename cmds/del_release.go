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
	"strings"

	"github.com/spf13/cobra"
	"gomodules.xyz/flags"
)

func NewCmdDeleteRelease() *cobra.Command {
	var src string

	cmd := &cobra.Command{
		Use:               "delete-release",
		Short:             "Delete releases from one GitHub repo",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			deleteRelease(src)
		},
	}

	cmd.Flags().StringVar(&src, "src", "", "Source owner/repo")

	return cmd
}

func deleteRelease(src string) {
	parts := strings.SplitN(src, "/", 2)
	if len(parts) != 2 {
		log.Fatalf("expected src to be owner/repo format, found %s", src)
	}
	srcOwner, srcRepo := parts[0], parts[1]

	// github client
	ctx := context.Background()
	client := newGitHubClient(ctx)

	srcReleases, err := ListReleases(ctx, client, srcOwner, srcRepo)
	if err != nil {
		log.Fatalln(err)
	}

	for _, srcRelease := range srcReleases {
		fmt.Println(srcRelease.GetTagName())
		_, err = client.Repositories.DeleteRelease(ctx, srcOwner, srcRepo, srcRelease.GetID())
		if err != nil {
			log.Fatal(err)
		}
	}
}
