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
	"github.com/spf13/cobra"
	"gomodules.xyz/flags"
)

func NewCmdUnprotectRepo() *cobra.Command {
	var (
		owner          string
		repo           string
		rules          []string
		deleteAllRules bool
		bypass         bool
	)

	cmd := &cobra.Command{
		Use:               "unprotect-repo",
		Short:             "Delete matching rulesets and branch protections from a repository",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			runUnprotectRepo(owner, repo, rules, deleteAllRules, bypass)
		},
	}

	cmd.Flags().StringVar(&owner, "owner", owner, "GitHub user or org name")
	cmd.Flags().StringVar(&repo, "repo", repo, "GitHub repository name")
	cmd.Flags().StringSliceVar(&rules, "rule", nil, "Rule name to delete (ruleset name or branch name, repeatable)")
	cmd.Flags().BoolVar(&deleteAllRules, "all-rules", false, "If true, delete all repository rulesets and branch protection rules")
	cmd.Flags().BoolVar(&bypass, "bypass", false, "If true, do not delete rules; allow bypassing on matched branch protection rules")
	_ = cmd.MarkFlagRequired("owner")
	_ = cmd.MarkFlagRequired("repo")

	return cmd
}
