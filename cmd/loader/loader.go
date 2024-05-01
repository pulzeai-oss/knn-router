package loader

import (
	"log"

	"github.com/pulzeai-oss/knn-router/internal/loader"
	"github.com/spf13/cobra"
)

type loaderOpts struct {
	DBPath         string
	pointsDataPath string
	scoresDataPath string
}

var opts loaderOpts

var LoaderCmd = &cobra.Command{
	Use:   "load",
	Short: "Write dataset to database",
	Run: func(cmd *cobra.Command, args []string) {
		loader := loader.NewLoader()
		if err := loader.LoadPoints(opts.pointsDataPath); err != nil {
			log.Fatalf("failed to load points: %v", err)
		}
		if err := loader.LoadScores(opts.scoresDataPath); err != nil {
			log.Fatalf("failed to load scores: %v", err)
		}
		if err := loader.SaveScores(opts.DBPath); err != nil {
			log.Fatalf("failed to write to DB: %v", err)
		}
	},
}

func init() {
	LoaderCmd.Flags().
		StringVar(&opts.pointsDataPath, "points-data-path", "", "Path to JSONL-formatted dataset containing points")
	LoaderCmd.Flags().
		StringVar(&opts.scoresDataPath, "scores-data-path", "", "Path to JSONL-formatted dataset containing target scores")
	LoaderCmd.Flags().
		StringVar(&opts.DBPath, "db-path", "scores.db", "The path to write Bolt database to")
}
