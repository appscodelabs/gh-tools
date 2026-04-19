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

	"github.com/google/go-github/v84/github"
)

func cacheOrgFreePlan(org string, isFree bool) {
	freeOrgs[org] = isFree
}

func repoSupportsProtection(ctx context.Context, client *github.Client, repo *github.Repository) (bool, string, error) {
	if repo == nil {
		return false, "repository not found", nil
	}
	if !repo.GetPrivate() {
		return true, "", nil
	}
	if repo.GetOwner().GetType() == OwnerTypeUser {
		return false, "private user repositories do not support this feature", nil
	}

	isFreeOrg, err := orgUsesFreePlan(ctx, client, repo.GetOwner().GetLogin())
	if err != nil {
		return false, "", err
	}
	if isFreeOrg {
		return false, "private repositories in GitHub Free organizations do not support this feature", nil
	}

	return true, "", nil
}

func orgUsesFreePlan(ctx context.Context, client *github.Client, org string) (bool, error) {
	if isFree, ok := freeOrgs[org]; ok {
		return isFree, nil
	}

	orgInfo, _, err := client.Organizations.Get(ctx, org)
	if err != nil {
		return false, err
	}

	isFree := orgInfo.GetPlan().GetName() == "free"
	cacheOrgFreePlan(org, isFree)
	return isFree, nil
}
