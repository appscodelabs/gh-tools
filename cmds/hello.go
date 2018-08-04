package cmds

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmdHello() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "hello",
		Short:             "Hello World",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Hello! Using gh-tools")
		},
	}

	return cmd
}
