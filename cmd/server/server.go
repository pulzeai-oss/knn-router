package server

import (
	"context"
	"log"

	"github.com/pulzeai-oss/knn-router/internal/server"
	"github.com/pulzeai-oss/knn-router/internal/teipb"
	"github.com/spf13/cobra"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type serverOpts struct {
	bindAddr     string
	embedAddr    string
	qdrantAddr   string
	scoresDBPath string
	topK         int
}

var opts serverOpts

var ServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the KNN-router server",
	Run: func(cmd *cobra.Command, args []string) {
		scoresDB, err := bolt.Open(opts.scoresDBPath, 0600, &bolt.Options{ReadOnly: true})
		if err != nil {
			log.Fatalf("Failed to open scores database: %v", err)
		}
		defer scoresDB.Close()

		embedConn, err := grpc.DialContext(
			context.Background(),
			opts.embedAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Fatalf("Failed to create connection to embedding server: %v", err)
		}
		defer embedConn.Close()

		infoClient := teipb.NewInfoClient(embedConn)
		infoResp, err := infoClient.Info(context.Background(), &teipb.InfoRequest{})
		if err != nil {
			log.Fatalf("Failed to get info from embedding server: %v", err)
		}

		qdrantConn, err := grpc.DialContext(
			context.Background(),
			opts.qdrantAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Fatalf("Failed to create connection to Qdrant server: %v", err)
		}
		defer qdrantConn.Close()

		svr := server.NewServer(
			embedConn,
			qdrantConn,
			scoresDB,
			opts.topK,
			int(infoResp.MaxInputLength),
		)
		if err := svr.ListenAndServe(opts.bindAddr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	},
}

func init() {
	ServerCmd.Flags().
		StringVarP(&opts.bindAddr, "bind-addr", "a", ":8888", "Address and port to bind the server to")
	ServerCmd.Flags().
		StringVarP(&opts.embedAddr, "embed-address", "e", "localhost:8889", "Address and port of the embedding inference server")
	ServerCmd.Flags().
		StringVarP(&opts.qdrantAddr, "qdrant-address", "q", "localhost:6334", "Address and port of the Qdrant server")
	ServerCmd.Flags().
		StringVarP(&opts.scoresDBPath, "scores-db-path", "s", "scores.db", "The path to the scores database file")
	ServerCmd.Flags().
		IntVarP(&opts.topK, "top-k", "k", 10, "The number of top hits to aggregate")
}
