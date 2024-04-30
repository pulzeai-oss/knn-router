package generate

import (
	"log"

	"github.com/pulzeai-oss/knn-router/internal/generate"
	"github.com/spf13/cobra"
)

type generateOpts struct {
	dataPath     string
	scoresDBPath string
}

var opts generateOpts

var GenerateCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate a Bolt DB from scores data",
	Run: func(cmd *cobra.Command, args []string) {
		if err := generate.LoadDB(opts.dataPath, opts.scoresDBPath); err != nil {
			log.Fatalf("Failed to load database: %v", err)
		}
	},
}

func init() {
	GenerateCmd.Flags().
		StringVarP(&opts.dataPath, "data-path", "d", "", "Path to JSONL-formatted dataset source")
	GenerateCmd.Flags().
		StringVarP(&opts.scoresDBPath, "scores-db-path", "s", "scores.db", "The path to the scores database file")
}
