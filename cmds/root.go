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
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	v "gomodules.xyz/x/version"
)

func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "gh-tools [command]",
		Short:             `GitHub tools by AppsCode`,
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			c.Flags().VisitAll(func(flag *pflag.Flag) {
				log.Printf("FLAG: --%s=%q", flag.Name, flag.Value)
			})
		},
	}
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	// ref: https://github.com/kubernetes/kubernetes/issues/17162#issuecomment-225596212
	_ = flag.CommandLine.Parse([]string{})

	cmd.AddCommand(NewCmdProtect())
	cmd.AddCommand(NewCmdStarReport())
	cmd.AddCommand(NewCmdListOrgs())
	cmd.AddCommand(NewCmdStopWatch())
	cmd.AddCommand(v.NewCmdVersion())
	cmd.AddCommand(NewCmdChangelog())
	cmd.AddCommand(NewCmdRelease())
	cmd.AddCommand(NewCmdAddLabels())

	return cmd
}
