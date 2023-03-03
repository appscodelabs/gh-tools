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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/appscodelabs/gh-tools/internal/git"

	"github.com/spf13/cobra"
	"gomodules.xyz/flags"
)

func NewCmdChangelog() *cobra.Command {
	var sort string
	var exclude []string

	cmd := &cobra.Command{
		Use:               "changelog",
		Short:             "generate changelog",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			flags.PrintFlags(c.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			runChangelog(sort, exclude)
		},
	}
	cmd.Flags().StringVar(&sort, "sort", "asc", "could either be asc, desc or empty")
	cmd.Flags().StringArrayVar(&exclude, "exclude", []string{"^docs:", "^test:"}, "commit messages matching the regexp listed here will be removed from the changelog")
	return cmd
}

func runChangelog(sort string, exclude []string) {
	entries, err := buildChangelog(sort, exclude)
	if err != nil {
		log.Fatal(err)
	}

	err = os.MkdirAll("dist", 0o755)
	if err != nil {
		log.Fatal(err)
	}

	path := filepath.Join("dist", "CHANGELOG.md")

	releaseNotes := fmt.Sprintf("## Changelog\n\n%v\n", strings.Join(entries, "\n"))

	err = os.WriteFile(path, []byte(releaseNotes), 0o644)
	if err != nil {
		log.Fatal(err)
	}
}

func buildChangelog(sort string, exclude []string) ([]string, error) {
	// need current tag
	tag, err := git.Clean(git.Run("tag", "-l", "--points-at", "HEAD"))
	if err != nil {
		return nil, err
	}

	log, err := getChangelog(tag)
	if err != nil {
		return nil, err
	}
	entries := strings.Split(log, "\n")
	entries = entries[0 : len(entries)-1]

	entries, err = filterEntries(exclude, entries)
	if err != nil {
		return entries, err
	}

	return sortEntries(sort, entries), nil
}

func filterEntries(filters, entries []string) ([]string, error) {
	for _, filter := range filters {
		r, err := regexp.Compile(filter)
		if err != nil {
			return entries, err
		}
		entries = remove(r, entries)
	}
	return entries, nil
}

func sortEntries(direction string, entries []string) []string {
	if direction == "" {
		return entries
	}

	result := make([]string, len(entries))
	copy(result, entries)
	sort.Slice(result, func(i, j int) bool {
		_, imsg := extractCommitInfo(result[i])
		_, jmsg := extractCommitInfo(result[j])
		if direction == "asc" {
			return strings.Compare(imsg, jmsg) < 0
		}
		return strings.Compare(imsg, jmsg) > 0
	})
	return result
}

//nolint:unparam
func extractCommitInfo(line string) (hash, msg string) {
	ss := strings.Split(line, " ")
	return ss[0], strings.Join(ss[1:], " ")
}

func getChangelog(tag string) (string, error) {
	prev, err := previous(tag)
	if err != nil {
		return "", err
	}
	if !prev.Tag {
		return gitLog(prev.SHA, tag)
	}
	return gitLog(fmt.Sprintf("%v..%v", prev.SHA, tag))
}

func previous(tag string) (result ref, err error) {
	result.Tag = true
	result.SHA, err = git.Clean(git.Run("describe", "--tags", "--abbrev=0", tag+"^"))
	if err != nil {
		result.Tag = false
		result.SHA, err = git.Clean(git.Run("rev-list", "--max-parents=0", "HEAD"))
	}
	return
}

func gitLog(refs ...string) (string, error) {
	args := []string{"log", "--pretty=oneline", "--abbrev-commit", "--no-decorate", "--no-color"}
	args = append(args, refs...)
	return git.Run(args...)
}

func remove(filter *regexp.Regexp, entries []string) (result []string) {
	for _, entry := range entries {
		_, msg := extractCommitInfo(entry)
		if !filter.MatchString(msg) {
			result = append(result, entry)
		}
	}
	return result
}

type ref struct {
	Tag bool
	SHA string
}
