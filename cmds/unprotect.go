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
	"log"
	"slices"
	"strings"
	"time"

	"github.com/google/go-github/v84/github"
	"github.com/spf13/cobra"
	"gomodules.xyz/flags"
	"gomodules.xyz/sets"
)

func NewCmdUnprotect() *cobra.Command {
	var (
		rules           []string
		includeFork     bool
		skipRepos       []string
		localShards     int
		localShardIndex int
	)

	cmd := &cobra.Command{
		Use:               "unprotect",
		Short:             "Delete matching rulesets from accessible repositories",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			runUnprotect(rules, includeFork, skipRepos, localShardIndex, localShards)
		},
	}

	cmd.Flags().StringSliceVar(&rules, "rule", nil, "Ruleset name to delete (repeatable)")
	cmd.Flags().BoolVar(&includeFork, "fork", false, "If true, include forked repos")
	cmd.Flags().StringSliceVar(&skipRepos, "skip", nil, "Skip owner/repository")
	cmd.Flags().IntVar(&localShards, "shards", -1, "Total number of shards")
	cmd.Flags().IntVar(&localShardIndex, "shard-index", -1, "Shard index to be processed")

	return cmd
}

func runUnprotect(rules []string, includeFork bool, skipRepos []string, localShardIndex, localShards int) {
	requestedRules := normalizeRules(rules)
	if len(requestedRules) == 0 {
		log.Println("WARNING: no --rule names provided, nothing to delete")
		return
	}

	ctx := context.Background()
	client := newGitHubClient(ctx)

	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("user:", user.GetLogin())

	opt := &github.RepositoryListByAuthenticatedUserOptions{
		Affiliation: "owner,organization_member",
		ListOptions: github.ListOptions{PerPage: 50},
	}
	repos, err := ListRepos(ctx, client, opt, includeFork)
	if err != nil {
		log.Fatalln(err)
	}

	repos = shardRepos(repos, localShardIndex, localShards)
	skipSet := sets.NewString(skipRepos...)
	log.Printf("Found %d repositories", len(repos))

	totalDeleted := 0
	for _, repo := range repos {
		if repo.GetOwner().GetType() == OwnerTypeUser {
			continue
		}
		if !repo.GetPermissions().GetAdmin() {
			continue
		}
		if skipSet.Has(repo.GetFullName()) {
			continue
		}

		deleted, err := deleteMatchingRepoRulesets(ctx, client, repo.GetOwner().GetLogin(), repo.GetName(), requestedRules)
		if err != nil {
			log.Fatalln(err)
		}
		totalDeleted += deleted
		time.Sleep(10 * time.Millisecond)
	}

	log.Printf("deleted %d matching ruleset(s) in total", totalDeleted)
}

func runUnprotectRepo(owner, repo string, rules []string) {
	requestedRules := normalizeRules(rules)
	if len(requestedRules) == 0 {
		log.Println("WARNING: no --rule names provided, nothing to delete")
		return
	}

	ctx := context.Background()
	client := newGitHubClient(ctx)

	r, err := GetRepo(ctx, client, owner, repo)
	if err != nil {
		log.Fatalln(err)
	}
	if r == nil {
		log.Printf("repository not found: %s/%s", owner, repo)
		return
	}

	deleted, err := deleteMatchingRepoRulesets(ctx, client, owner, repo, requestedRules)
	if err != nil {
		log.Fatalln(err)
	}
	if deleted == 0 {
		log.Printf("no matching rulesets found in %s/%s for names: %s", owner, repo, strings.Join(sortedRuleNames(requestedRules), ", "))
		return
	}
	log.Printf("deleted %d matching ruleset(s)", deleted)
}

func runUnprotectOrg(org string, includeForks bool, skipList []string, rules []string) {
	requestedRules := normalizeRules(rules)
	if len(requestedRules) == 0 {
		log.Println("WARNING: no --rule names provided, nothing to delete")
		return
	}
	if org == "" {
		log.Fatal("--org flag is required")
	}

	ctx := context.Background()
	client := newGitHubClient(ctx)

	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("user:", user.GetLogin())

	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 50},
	}
	repos, err := ListOrgRepos(ctx, client, org, opt, includeForks)
	if err != nil {
		log.Fatalln(err)
	}

	skipRepos := sets.NewString(skipList...)
	log.Printf("Found %d repositories in org %s", len(repos), org)

	totalDeleted := 0
	for _, repo := range repos {
		if !repo.GetPermissions().GetAdmin() {
			log.Printf("Skipping %s (no admin permission)", repo.GetFullName())
			continue
		}
		if skipRepos.Has(repo.GetName()) {
			log.Printf("Skipping %s (in skip list)", repo.GetFullName())
			continue
		}

		deleted, err := deleteMatchingRepoRulesets(ctx, client, org, repo.GetName(), requestedRules)
		if err != nil {
			log.Fatalln(err)
		}
		totalDeleted += deleted
		time.Sleep(10 * time.Millisecond)
	}

	log.Printf("deleted %d matching ruleset(s) in org %s", totalDeleted, org)
}

func listRepoRulesets(ctx context.Context, client *github.Client, owner, repo string) ([]*github.RepositoryRuleset, error) {
	opt := &github.RepositoryListRulesetsOptions{
		IncludesParents: github.Ptr(false),
		ListOptions:     github.ListOptions{PerPage: 100},
	}

	var out []*github.RepositoryRuleset
	for {
		items, resp, err := client.Repositories.GetAllRulesets(ctx, owner, repo, opt)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return out, nil
}

func deleteMatchingRepoRulesets(ctx context.Context, client *github.Client, owner, repo string, requestedRules map[string]struct{}) (int, error) {
	rulesets, err := listRepoRulesets(ctx, client, owner, repo)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, rs := range rulesets {
		if rs == nil || rs.ID == nil {
			continue
		}
		if _, ok := requestedRules[rs.Name]; !ok {
			continue
		}

		log.Printf("[DELETE] %s/%s ruleset %q (%d)", owner, repo, rs.Name, rs.GetID())
		if _, err := client.Repositories.DeleteRuleset(ctx, owner, repo, rs.GetID()); err != nil {
			return deleted, err
		}
		deleted++
	}

	return deleted, nil
}

func normalizeRules(rules []string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		out[rule] = struct{}{}
	}
	return out
}

func sortedRuleNames(rules map[string]struct{}) []string {
	if len(rules) == 0 {
		return nil
	}

	names := make([]string, 0, len(rules))
	for name := range rules {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func shardRepos(in []*github.Repository, shardIndex, shards int) []*github.Repository {
	if shardIndex < 0 || shards < 1 {
		return in
	}

	sorted := append([]*github.Repository(nil), in...)
	slices.SortFunc(sorted, func(a, b *github.Repository) int {
		return strings.Compare(a.GetFullName(), b.GetFullName())
	})

	itemsPerShard := (len(sorted) + shards - 1) / shards
	if itemsPerShard == 0 {
		return nil
	}

	start := shardIndex * itemsPerShard
	if start >= len(sorted) {
		return nil
	}
	end := min(start+itemsPerShard, len(sorted))
	return sorted[start:end]
}
