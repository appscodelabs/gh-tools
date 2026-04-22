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
	"errors"
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
		deleteAllRules  bool
		bypass          bool
		includeFork     bool
		skipRepos       []string
		localShards     int
		localShardIndex int
	)

	cmd := &cobra.Command{
		Use:               "unprotect",
		Short:             "Delete matching rulesets and branch protections from accessible repositories",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			runUnprotect(rules, deleteAllRules, bypass, includeFork, skipRepos, localShardIndex, localShards)
		},
	}

	cmd.Flags().StringSliceVar(&rules, "rule", nil, "Rule name to delete (ruleset name or branch name, repeatable)")
	cmd.Flags().BoolVar(&deleteAllRules, "all-rules", false, "If true, delete all repository rulesets and branch protection rules")
	cmd.Flags().BoolVar(&bypass, "bypass", false, "If true, do not delete rules; allow bypassing on matched branch protection rules")
	cmd.Flags().BoolVar(&includeFork, "fork", false, "If true, include forked repos")
	cmd.Flags().StringSliceVar(&skipRepos, "skip", nil, "Skip owner/repository")
	cmd.Flags().IntVar(&localShards, "shards", -1, "Total number of shards")
	cmd.Flags().IntVar(&localShardIndex, "shard-index", -1, "Shard index to be processed")

	return cmd
}

func runUnprotect(rules []string, deleteAllRules bool, bypass bool, includeFork bool, skipRepos []string, localShardIndex, localShards int) {
	requestedRules := normalizeRules(rules)
	if !deleteAllRules && len(requestedRules) == 0 {
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

	totalRulesetsDeleted := 0
	totalBranchProtectionsDeleted := 0
	for _, repo := range repos {
		if repo.GetOwner().GetType() == OwnerTypeUser {
			continue
		}
		if !repo.GetPermissions().GetAdmin() {
			continue
		}
		supported, reason, err := repoSupportsProtection(ctx, client, repo)
		if err != nil {
			log.Fatalln(err)
		}
		if !supported {
			log.Printf("Skipping %s (%s)", repo.GetFullName(), reason)
			continue
		}
		if skipSet.Has(repo.GetFullName()) {
			continue
		}
		if bypass {
			branchProtectionsUpdated, err := relaxRepoBranchProtectionBypass(ctx, client, repo, requestedRules, deleteAllRules)
			if err != nil {
				log.Fatalln(err)
			}
			totalBranchProtectionsDeleted += branchProtectionsUpdated
			time.Sleep(10 * time.Millisecond)
			continue
		}

		rulesetsDeleted, err := deleteMatchingRepoRulesets(ctx, client, repo.GetOwner().GetLogin(), repo.GetName(), requestedRules, deleteAllRules)
		if err != nil {
			log.Fatalln(err)
		}
		totalRulesetsDeleted += rulesetsDeleted

		branchProtectionsDeleted, err := deleteRepoBranchProtections(ctx, client, repo, requestedRules, deleteAllRules)
		if err != nil {
			log.Fatalln(err)
		}
		totalBranchProtectionsDeleted += branchProtectionsDeleted
		time.Sleep(10 * time.Millisecond)
	}
	if bypass {
		log.Printf("updated %d branch protection rule(s) to allow bypass", totalBranchProtectionsDeleted)
		return
	}
	log.Printf("deleted %d matching ruleset(s) and %d branch protection rule(s) in total", totalRulesetsDeleted, totalBranchProtectionsDeleted)
}

func runUnprotectRepo(owner, repo string, rules []string, deleteAllRules bool, bypass bool) {
	requestedRules := normalizeRules(rules)
	if !deleteAllRules && len(requestedRules) == 0 {
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

	supported, reason, err := repoSupportsProtection(ctx, client, r)
	if err != nil {
		log.Fatalln(err)
	}
	if !supported {
		log.Printf("Skipping %s (%s)", r.GetFullName(), reason)
		return
	}

	if bypass {
		updated, err := relaxRepoBranchProtectionBypass(ctx, client, r, requestedRules, deleteAllRules)
		if err != nil {
			log.Fatalln(err)
		}
		if updated == 0 {
			log.Printf("no matching branch protection rules found in %s/%s", owner, repo)
			return
		}
		log.Printf("updated %d branch protection rule(s) to allow bypass", updated)
		return
	}

	rulesetsDeleted, err := deleteMatchingRepoRulesets(ctx, client, owner, repo, requestedRules, deleteAllRules)
	if err != nil {
		log.Fatalln(err)
	}

	branchProtectionsDeleted, err := deleteRepoBranchProtections(ctx, client, r, requestedRules, deleteAllRules)
	if err != nil {
		log.Fatalln(err)
	}

	if rulesetsDeleted == 0 {
		if deleteAllRules {
			log.Printf("no rulesets found in %s/%s", owner, repo)
		} else {
			log.Printf("no matching rulesets found in %s/%s for names: %s", owner, repo, strings.Join(sortedRuleNames(requestedRules), ", "))
		}
	}

	if rulesetsDeleted == 0 && branchProtectionsDeleted == 0 {
		log.Printf("nothing to unprotect in %s/%s", owner, repo)
		return
	}
	log.Printf("deleted %d matching ruleset(s) and %d branch protection rule(s)", rulesetsDeleted, branchProtectionsDeleted)
}

func runUnprotectOrg(org string, includeForks bool, skipList []string, rules []string, deleteAllRules bool, bypass bool) {
	requestedRules := normalizeRules(rules)
	if !deleteAllRules && len(requestedRules) == 0 {
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

	if _, err := orgUsesFreePlan(ctx, client, org); err != nil {
		log.Fatalln(err)
	}

	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 50},
	}
	repos, err := ListOrgRepos(ctx, client, org, opt, includeForks)
	if err != nil {
		log.Fatalln(err)
	}

	skipRepos := sets.NewString(skipList...)
	log.Printf("Found %d repositories in org %s", len(repos), org)

	totalRulesetsDeleted := 0
	totalBranchProtectionsDeleted := 0
	for _, repo := range repos {
		if !repo.GetPermissions().GetAdmin() {
			log.Printf("Skipping %s (no admin permission)", repo.GetFullName())
			continue
		}
		supported, reason, err := repoSupportsProtection(ctx, client, repo)
		if err != nil {
			log.Fatalln(err)
		}
		if !supported {
			log.Printf("Skipping %s (%s)", repo.GetFullName(), reason)
			continue
		}
		if skipRepos.Has(repo.GetName()) {
			log.Printf("Skipping %s (in skip list)", repo.GetFullName())
			continue
		}
		if bypass {
			branchProtectionsUpdated, err := relaxRepoBranchProtectionBypass(ctx, client, repo, requestedRules, deleteAllRules)
			if err != nil {
				log.Fatalln(err)
			}
			totalBranchProtectionsDeleted += branchProtectionsUpdated
			time.Sleep(10 * time.Millisecond)
			continue
		}

		rulesetsDeleted, err := deleteMatchingRepoRulesets(ctx, client, org, repo.GetName(), requestedRules, deleteAllRules)
		if err != nil {
			log.Fatalln(err)
		}
		totalRulesetsDeleted += rulesetsDeleted

		branchProtectionsDeleted, err := deleteRepoBranchProtections(ctx, client, repo, requestedRules, deleteAllRules)
		if err != nil {
			log.Fatalln(err)
		}
		totalBranchProtectionsDeleted += branchProtectionsDeleted
		time.Sleep(10 * time.Millisecond)
	}
	if bypass {
		log.Printf("updated %d branch protection rule(s) to allow bypass in org %s", totalBranchProtectionsDeleted, org)
		return
	}
	log.Printf("deleted %d matching ruleset(s) and %d branch protection rule(s) in org %s", totalRulesetsDeleted, totalBranchProtectionsDeleted, org)
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

func deleteMatchingRepoRulesets(ctx context.Context, client *github.Client, owner, repo string, requestedRules map[string]struct{}, deleteAllRules bool) (int, error) {
	rulesets, err := listRepoRulesets(ctx, client, owner, repo)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, rs := range rulesets {
		if rs == nil || rs.ID == nil {
			continue
		}
		if !deleteAllRules {
			if _, ok := requestedRules[rs.Name]; !ok {
				continue
			}
		}

		if deleteAllRules {
			log.Printf("[DELETE] %s/%s ruleset %q (%d) [all-rules]", owner, repo, rs.Name, rs.GetID())
		} else {
			log.Printf("[DELETE] %s/%s ruleset %q (%d)", owner, repo, rs.Name, rs.GetID())
		}
		if _, err := client.Repositories.DeleteRuleset(ctx, owner, repo, rs.GetID()); err != nil {
			return deleted, err
		}
		deleted++
	}

	return deleted, nil
}

func deleteRepoBranchProtections(ctx context.Context, client *github.Client, repo *github.Repository, requestedRules map[string]struct{}, deleteAllRules bool) (int, error) {
	branches, err := ListBranches(ctx, client, repo)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, branch := range branches {
		name := branch.GetName()
		if !deleteAllRules {
			if _, ok := requestedRules[name]; !ok {
				continue
			}
		}

		if _, err := client.Repositories.RemoveBranchProtection(ctx, repo.Owner.GetLogin(), repo.GetName(), name); err != nil {
			if e, ok := err.(*github.ErrorResponse); ok && e.Response != nil && e.Response.StatusCode == 404 {
				continue
			}
			return deleted, err
		}
		if deleteAllRules {
			log.Printf("[DELETE] %s/%s branch protection %q [all-rules]", repo.Owner.GetLogin(), repo.GetName(), name)
		} else {
			log.Printf("[DELETE] %s/%s branch protection %q", repo.Owner.GetLogin(), repo.GetName(), name)
		}
		deleted++
	}

	return deleted, nil
}

func relaxRepoBranchProtectionBypass(ctx context.Context, client *github.Client, repo *github.Repository, requestedRules map[string]struct{}, deleteAllRules bool) (int, error) {
	branches, err := ListBranches(ctx, client, repo)
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, branch := range branches {
		name := branch.GetName()
		if !deleteAllRules {
			if _, ok := requestedRules[name]; !ok {
				continue
			}
		}

		if _, err := client.Repositories.RemoveAdminEnforcement(ctx, repo.Owner.GetLogin(), repo.GetName(), name); err != nil {
			if shouldIgnoreBypassUpdateError(err) {
				continue
			}
			return updated, err
		}
		if deleteAllRules {
			log.Printf("[BYPASS] %s/%s branch protection %q [all-rules]", repo.Owner.GetLogin(), repo.GetName(), name)
		} else {
			log.Printf("[BYPASS] %s/%s branch protection %q", repo.Owner.GetLogin(), repo.GetName(), name)
		}
		updated++
	}

	return updated, nil
}

func shouldIgnoreBypassUpdateError(err error) bool {
	if errors.Is(err, github.ErrBranchNotProtected) {
		return true
	}

	e, ok := err.(*github.ErrorResponse)
	if !ok || e.Response == nil {
		return false
	}
	if e.Response.StatusCode == 404 {
		return true
	}
	if e.Response.StatusCode == 422 {
		msg := strings.ToLower(e.Message)
		if strings.Contains(msg, "not protected") || strings.Contains(msg, "enforce_admins") {
			return true
		}
	}
	return false
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
