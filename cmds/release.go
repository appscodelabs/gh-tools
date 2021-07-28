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
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/appscodelabs/gh-tools/internal/git"

	"github.com/google/go-github/v35/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"gomodules.xyz/flags"
)

func NewCmdRelease() *cobra.Command {
	var owner, repo string
	var draft, prerelease bool

	cmd := &cobra.Command{
		Use:               "release",
		Short:             "create GitHub release and upload artifacts",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			runRelease(owner, repo, draft, prerelease)
		},
	}

	cmd.Flags().StringVar(&owner, "owner", "", "Owner of the repository.")
	cmd.Flags().StringVar(&repo, "repo", "", "Name of the repository.")
	cmd.Flags().BoolVar(&draft, "draft", true, "If set to true, will not auto-publish the release.")
	cmd.Flags().BoolVar(&prerelease, "prerelease", false, "If set to true, will mark the release as not ready for production.")

	return cmd
}

func runRelease(owner, repo string, draft, prerelease bool) {
	if owner == "" {
		log.Fatal("Owner name can't be empty")
	}
	if repo == "" {
		log.Fatal("Repository name can't be empty")
	}

	token, found := os.LookupEnv("GH_TOOLS_TOKEN")
	if !found {
		log.Fatalln("GH_TOOLS_TOKEN env var is not set")
	}

	//github client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	tag, err := git.Clean(git.Run("tag", "-l", "--points-at", "HEAD"))
	if err != nil {
		log.Fatal(err)
	}

	buff, err := ioutil.ReadFile("dist/CHANGELOG.md")
	if err != nil {
		log.Fatal(err)
	}
	releaseNote := string(buff)

	release := &github.RepositoryRelease{
		TagName:    &tag,
		Name:       &tag,
		Body:       &releaseNote,
		Draft:      &draft,
		Prerelease: &prerelease,
	}

	//create release
	release, _, err = client.Repositories.CreateRelease(ctx, owner, repo, release)
	if err != nil {
		log.Fatal(err)
	}

	//upload all files present in dist
	dirToWalk := "dist"
	subDirToSkip := "local"
	err = filepath.Walk(dirToWalk, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == subDirToSkip {
			return filepath.SkipDir
		}
		if !info.IsDir() && info.Name() != "CHANGELOG.md" {
			log.Println("uploading ", info.Name())

			file, err := os.Open(path)
			if err != nil {
				log.Fatal(err)
			}

			//upload artifacts
			_, _, err = client.Repositories.UploadReleaseAsset(
				ctx,
				owner,
				repo,
				*release.ID,
				&github.UploadOptions{
					Name: info.Name(),
				},
				file,
			)

			if err != nil {
				log.Fatal(err)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
