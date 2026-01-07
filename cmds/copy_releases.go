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
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v81/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"gomodules.xyz/flags"
)

func NewCmdCopyRelease() *cobra.Command {
	var src, dest string

	cmd := &cobra.Command{
		Use:               "copy-release",
		Short:             "Copy releases from one GitHub repo to another repo",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			copyRelease(src, dest)
		},
	}

	cmd.Flags().StringVar(&src, "src", "", "Source owner/repo")
	cmd.Flags().StringVar(&dest, "dest", "", "Destination owner/repo")

	return cmd
}

func copyRelease(src, dest string) {
	parts := strings.SplitN(src, "/", 2)
	if len(parts) != 2 {
		log.Fatalf("expected src to be owner/repo format, found %s", src)
	}
	srcOwner, srcRepo := parts[0], parts[1]

	parts = strings.SplitN(dest, "/", 2)
	if len(parts) != 2 {
		log.Fatalf("expected dest to be owner/repo format, found %s", dest)
	}
	destOwner, destRepo := parts[0], parts[1]

	token, found := os.LookupEnv("GH_TOOLS_TOKEN")
	if !found {
		log.Fatalln("GH_TOOLS_TOKEN env var is not set")
	}

	// github client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	var buf bytes.Buffer

	srcReleases, err := ListReleases(ctx, client, srcOwner, srcRepo)
	if err != nil {
		log.Fatalln(err)
	}
	destReleases, err := ListReleases(ctx, client, destOwner, destRepo)
	if err != nil {
		log.Fatalln(err)
	}
	destReleaseMap := make(map[string]*github.RepositoryRelease)
	for _, destRelease := range destReleases {
		destReleaseMap[destRelease.GetTagName()] = destRelease
	}

	for _, srcRelease := range srcReleases {
		fmt.Println(srcRelease.GetTagName())
		var destRelease *github.RepositoryRelease

		// get release and add any missing content
		if existingRelease, ok := destReleaseMap[srcRelease.GetTagName()]; !ok {
			// create destRelease
			destRelease = &github.RepositoryRelease{
				TagName:    srcRelease.TagName,
				Name:       srcRelease.Name,
				Body:       srcRelease.Body,
				Draft:      srcRelease.Draft,
				Prerelease: srcRelease.Prerelease,
			}
			destRelease, _, err = client.Repositories.CreateRelease(ctx, destOwner, destRepo, destRelease)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			destRelease = existingRelease
		}

		assetMap := make(map[string]*github.ReleaseAsset)
		for _, asset := range destRelease.Assets {
			assetMap[asset.GetName()] = asset
		}

		for _, asset := range srcRelease.Assets {
			if _, ok := assetMap[asset.GetName()]; ok {
				continue // already exists, continue
			}

			fmt.Println(asset.GetBrowserDownloadURL())

			dir := filepath.Join("/tmp", "github.com", srcOwner, srcRepo, srcRelease.GetTagName())
			if err = os.MkdirAll(dir, 0o755); err != nil {
				log.Fatalln(err)
			}
			buf.Reset()

			resp, err := http.Get(asset.GetBrowserDownloadURL())
			if err != nil {
				log.Fatalln(err)
			}
			if _, err = io.Copy(&buf, resp.Body); err != nil {
				log.Fatalln(err)
			}
			_ = resp.Body.Close()

			if err = os.WriteFile(filepath.Join(dir, asset.GetName()), buf.Bytes(), 0o644); err != nil {
				log.Fatalln(err)
			}

			file, err := os.Open(filepath.Join(dir, asset.GetName()))
			if err != nil {
				log.Fatal(err)
			}

			// upload artifacts
			_, _, err = client.Repositories.UploadReleaseAsset(
				ctx,
				destOwner,
				destRepo,
				*destRelease.ID,
				&github.UploadOptions{
					Name: asset.GetName(),
				},
				file,
			)
			if err != nil {
				log.Fatal(err)
			}
			_ = file.Close()
		}
	}
}

func ListReleases(ctx context.Context, client *github.Client, owner, repo string) ([]*github.RepositoryRelease, error) {
	opt := &github.ListOptions{
		PerPage: 100,
	}

	var result []*github.RepositoryRelease
	for {
		branch, resp, err := client.Repositories.ListReleases(ctx, owner, repo, opt)
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

		result = append(result, branch...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return result, nil
}
