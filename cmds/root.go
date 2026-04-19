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
	"flag"

	"github.com/spf13/cobra"
	v "gomodules.xyz/x/version"
)

func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "gh-tools [command]",
		Short:             `GitHub tools by AppsCode`,
		DisableAutoGenTag: true,
	}
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	// ref: https://github.com/kubernetes/kubernetes/issues/17162#issuecomment-225596212
	_ = flag.CommandLine.Parse([]string{})

	cmd.AddCommand(NewCmdAddLabels())
	cmd.AddCommand(NewCmdChangelog())
	cmd.AddCommand(NewCmdCopyRelease())
	cmd.AddCommand(NewCmdDeletePackage())
	cmd.AddCommand(NewCmdDeleteRelease())
	cmd.AddCommand(NewCmdDependabot())
	cmd.AddCommand(NewCmdListOrgs())
	cmd.AddCommand(NewCmdListRepos())
	cmd.AddCommand(NewCmdProtect())
	cmd.AddCommand(NewCmdProtectOrg())
	cmd.AddCommand(NewCmdProtectRepo())
	cmd.AddCommand(NewCmdRelease())
	cmd.AddCommand(NewCmdStarReport())
	cmd.AddCommand(NewCmdStopWatch())
	cmd.AddCommand(NewCmdUnprotect())
	cmd.AddCommand(NewCmdUnprotectOrg())
	cmd.AddCommand(NewCmdUnprotectRepo())
	cmd.AddCommand(v.NewCmdVersion())

	return cmd
}
