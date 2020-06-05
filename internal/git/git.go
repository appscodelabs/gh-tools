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

// Package git provides an integration with the git command
package git

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/apex/log"
)

// IsRepo returns true if current folder is a git repository
func IsRepo() bool {
	out, err := Run("rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// Run runs a git command and returns its output or errors
func Run(args ...string) (string, error) {
	// TODO: use exex.CommandContext here and refactor.
	/* #nosec */
	var cmd = exec.Command("git", args...)
	log.WithField("args", args).Debug("running git")
	bts, err := cmd.CombinedOutput()
	log.WithField("output", string(bts)).
		Debug("git result")
	if err != nil {
		return "", errors.New(string(bts))
	}
	return string(bts), nil
}

// Clean the output
func Clean(output string, err error) (string, error) {
	output = strings.Replace(strings.Split(output, "\n")[0], "'", "", -1)
	if err != nil {
		err = errors.New(strings.TrimSuffix(err.Error(), "\n"))
	}
	return output, err
}
