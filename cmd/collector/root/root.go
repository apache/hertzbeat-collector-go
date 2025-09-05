package root

import (
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/cmd"

	"github.com/spf13/cobra"
)

func GetRootCommand() *cobra.Command {

	c := &cobra.Command{
		Use:   "hertzbeat-collector-go",
		Short: "HertzBeat Collector Go",
		Long:  "Apache Hertzbeat Collector Go Impl",
	}

	c.AddCommand(cmd.VersionCommand())
	c.AddCommand(cmd.ServerCommand())

	return c
}
