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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/go-github/v81/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"gomodules.xyz/flags"
	"gomodules.xyz/pointer"
	"gomodules.xyz/sets"
)

func NewCmdDeletePackage() *cobra.Command {
	var (
		org string
		pkg string
		tag string
	)
	cmd := &cobra.Command{
		Use:               "delete-package",
		Short:             "Delete packages from ghcr.io",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			deletePackage(org, pkg, tag)
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Name of org")
	cmd.Flags().StringVar(&pkg, "pkg", "", "Name of package")
	cmd.Flags().StringVar(&tag, "tag", "", "Tag of package")

	return cmd
}

func deletePackage(org, pkg, tag string) {
	token, found := os.LookupEnv("GH_TOOLS_TOKEN")
	if !found {
		log.Fatalln("GH_TOOLS_TOKEN env var is not set")
	}

	// github client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	if pkg == "" {
		deleteAllOrgPackages(ctx, client, org)
	} else {
		deletePackageVersion(ctx, client, org, pkg, tag)
	}
}

func deleteAllOrgPackages(ctx context.Context, client *github.Client, org string) {
	pkgs1, err := ListPackages(ctx, client, org, "public")
	if err != nil {
		log.Fatalln(err)
	}
	pkgs2, err := ListPackages(ctx, client, org, "private")
	if err != nil {
		log.Fatalln(err)
	}
	for _, pkg := range append(pkgs1, pkgs2...) {
		fmt.Println("deleting package", pkg.GetName())
		_, err = client.Organizations.DeletePackage(ctx, org, pkg.GetPackageType(), pkg.GetName())
		if err != nil {
			log.Fatal(err)
		}
	}
}

func deletePackageVersion(ctx context.Context, client *github.Client, org, pkg, tag string) {
	versions, err := ListPackageVersions(ctx, client, org, pkg)
	if err != nil {
		log.Fatalln(err)
	}

	/*
	  {
	    "id": 78461757,
	    "name": "sha256:f4e77fe6e5b8592ae5c1f1f1dee0717359c7eb7990530724c8f04dafcfd2fef1",
	    "url": "https://api.github.com/orgs/voyagermesh/packages/container/haproxy/versions/78461757",
	    "package_html_url": "https://github.com/orgs/voyagermesh/packages/container/package/haproxy",
	    "created_at": "2023-03-19T08:11:27Z",
	    "updated_at": "2023-03-19T08:11:27Z",
	    "html_url": "https://github.com/orgs/voyagermesh/packages/container/haproxy/78461757",
	    "metadata": {
	      "package_type": "container",
	      "container": {
	        "tags": [
	          "2.7-debian",
	          "2.7.5-debian"
	        ]
	      }
	    }
	  }
	*/
	for _, ver := range versions {
		tags := sets.NewString()
		if ver.Metadata != nil {
			var md github.PackageMetadata
			if err := json.Unmarshal(ver.Metadata, &md); err == nil && md.Container != nil {
				tags.Insert(md.Container.Tags...)
			}
		}

		// PackageMetadata
		if tags.Has(tag) {
			_, err = client.Organizations.PackageDeleteVersion(ctx, org, "container", pkg, ver.GetID())
			if err != nil {
				log.Fatal(err)
			}
			break
		}
	}
}

func ListPackages(ctx context.Context, client *github.Client, owner, visibility string) ([]*github.Package, error) {
	opt := &github.PackageListOptions{
		PackageType: pointer.StringP("container"),
		Visibility:  pointer.StringP(visibility),
		State:       pointer.StringP("active"),
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var result []*github.Package
	for {
		versions, resp, err := client.Organizations.ListPackages(ctx, owner, opt)
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

		result = append(result, versions...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return result, nil
}

func ListPackageVersions(ctx context.Context, client *github.Client, owner, pkg string) ([]*github.PackageVersion, error) {
	opt := &github.PackageListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var result []*github.PackageVersion
	for {
		versions, resp, err := client.Organizations.PackageGetAllVersions(ctx, owner, "container", pkg, opt)
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

		result = append(result, versions...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return result, nil
}
