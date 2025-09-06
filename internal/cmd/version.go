package cmd

import (
	"github.com/spf13/cobra"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/cmd/version"
)

func VersionCommand() *cobra.Command {

	var output string

	cmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"versions", "v"},
		Short:   "Show versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return version.Print(cmd.OutOrStdout(), output)
		},
	}

	cmd.PersistentFlags().StringVarP(&output, "output", "o", "", "One of 'yaml' or 'json'")

	return cmd
}
