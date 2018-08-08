package cmds

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/appscodelabs/gh-tools/internal"
	"github.com/spf13/cobra"
)

func NewCmdChangelog() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "changelog",
		Short:             "generate changelog",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			runChangelog()
		},
	}
	return cmd
}

func runChangelog() {
	entries, err := buildChangelog()
	if err != nil {
		log.Fatal(err)
	}

	err = os.MkdirAll("dist", 0755)
	if err != nil {
		log.Fatal(err)
	}

	var path = filepath.Join("dist", "CHANGELOG.md")

	releaseNotes := fmt.Sprintf("## Changelog\n\n%v\n", strings.Join(entries, "\n"))

	err = ioutil.WriteFile(path, []byte(releaseNotes), 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func sortEntries(entries []string) []string {
	direction := "asc"
	var result = make([]string, len(entries))
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

func extractCommitInfo(line string) (hash, msg string) {
	ss := strings.Split(line, " ")
	return ss[0], strings.Join(ss[1:], " ")
}

func buildChangelog() ([]string, error) {
	//need current tag
	tag, err := git.Clean(git.Run("tag", "-l", "--points-at", "HEAD"))
	if err != nil {
		return nil, err
	}

	log, err := getChangelog(tag)
	if err != nil {
		return nil, err
	}
	var entries = strings.Split(log, "\n")
	entries = entries[0 : len(entries)-1]

	entries, err = filterEntries(entries)
	if err != nil {
		return entries, err
	}

	return sortEntries(entries), nil
}

func filterEntries(entries []string) ([]string, error) {
	filters := []string{"^docs:", "^test:"}
	for _, filter := range filters {
		r, err := regexp.Compile(filter)
		if err != nil {
			return entries, err
		}
		entries = remove(r, entries)
	}
	return entries, nil
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
	var args = []string{"log", "--pretty=oneline", "--abbrev-commit", "--no-decorate", "--no-color"}
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
