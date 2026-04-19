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
	"slices"
	"strings"

	"github.com/google/go-github/v84/github"
	"github.com/spf13/cobra"
	"gomodules.xyz/flags"
)

func NewCmdUnprotect() *cobra.Command {
	var (
		owner string
		repo  string
		rules []string
	)

	cmd := &cobra.Command{
		Use:               "unprotect",
		Short:             "Delete repository rulesets by name",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			runUnprotect(owner, repo, rules)
		},
	}

	cmd.Flags().StringVar(&owner, "owner", owner, "GitHub user or org name")
	cmd.Flags().StringVar(&repo, "repo", repo, "GitHub repository name")
	cmd.Flags().StringSliceVar(&rules, "rule", nil, "Ruleset name to delete (repeatable)")
	_ = cmd.MarkFlagRequired("owner")
	_ = cmd.MarkFlagRequired("repo")

	return cmd
}

func runUnprotect(owner, repo string, rules []string) {
	requestedRules := normalizeRules(rules)
	if len(requestedRules) == 0 {
		log.Println("WARNING: no --rule names provided, nothing to delete")
		return
	}

	ctx := context.Background()
	client := newGitHubClient(ctx)

	rulesets, err := listRepoRulesets(ctx, client, owner, repo)
	if err != nil {
		log.Fatalln(err)
	}

	deleted := 0
	for _, rs := range rulesets {
		if rs == nil || rs.ID == nil {
			continue
		}
		if _, ok := requestedRules[rs.Name]; !ok {
			continue
		}

		fmt.Printf("[DELETE] %s/%s ruleset %q (%d)\n", owner, repo, rs.Name, rs.GetID())
		if _, err := client.Repositories.DeleteRuleset(ctx, owner, repo, rs.GetID()); err != nil {
			log.Fatalln(err)
		}
		deleted++
	}

	if deleted == 0 {
		log.Printf("no matching rulesets found in %s/%s for names: %s", owner, repo, strings.Join(sortedRuleNames(requestedRules), ", "))
		return
	}

	log.Printf("deleted %d matching ruleset(s)", deleted)
}

func listRepoRulesets(ctx context.Context, client *github.Client, owner, repo string) ([]*github.RepositoryRuleset, error) {
	opt := &github.RepositoryListRulesetsOptions{
		IncludesParents: github.Ptr(false),
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
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
