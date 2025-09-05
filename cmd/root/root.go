package root

import (
	"github.com/spf13/cobra"

	"hertzbeat.apache.org/hertzbeat-collector-go/internal/cmd"
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
