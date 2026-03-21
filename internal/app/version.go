package app

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "0.0.2"

var vcmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintf(cmd.OutOrStdout(), "Sage current version is %s\n", version)
		return nil
	},
}

func versionCmd() *cobra.Command {
	return vcmd
}
