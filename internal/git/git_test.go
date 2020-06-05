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

package git

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGit(t *testing.T) {
	out, err := Run("status")
	assert.NoError(t, err)
	assert.NotEmpty(t, out)

	out, err = Run("command-that-dont-exist")
	assert.Error(t, err)
	assert.Empty(t, out)
	assert.Equal(
		t,
		"git: 'command-that-dont-exist' is not a git command. See 'git --help'.\n",
		err.Error(),
	)
}

func TestRepo(t *testing.T) {
	assert.True(t, IsRepo(), "goreleaser folder should be a git repo")

	assert.NoError(t, os.Chdir(os.TempDir()))
	assert.False(t, IsRepo(), os.TempDir()+" folder should be a git repo")
}

func TestClean(t *testing.T) {
	out, err := Clean("asdasd 'ssadas'\nadasd", nil)
	assert.NoError(t, err)
	assert.Equal(t, "asdasd ssadas", out)

	out, err = Clean(Run("command-that-dont-exist"))
	assert.Error(t, err)
	assert.Empty(t, out)
	assert.Equal(
		t,
		"git: 'command-that-dont-exist' is not a git command. See 'git --help'.",
		err.Error(),
	)
}
