package cmd

import (
	"github.com/pulzeai-oss/knn-router/cmd/loader"
	"github.com/pulzeai-oss/knn-router/cmd/server"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "knn-router [subcommand]",
	Short: "The CLI for KNN-Router",
}

func init() {
	rootCmd.AddCommand(server.ServerCmd)
	rootCmd.AddCommand(loader.LoaderCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
