package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"

	"github.com/pulzeai-oss/knn-router/internal/scorespb"
	"github.com/pulzeai-oss/knn-router/internal/teipb"
	qdrant "github.com/qdrant/go-client/qdrant"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

const (
	PointsCollection = "main"
)

type TruncateStrategy uint8

const (
	Head TruncateStrategy = iota + 1
	Tail
	Middle
	Ends
)

type Request struct {
	Prompt           string           `json:"prompt"`
	TruncateStrategy TruncateStrategy `json:"truncate_strategy"`
}

type Score struct {
	Target string  `json:"target"`
	Score  float32 `json:"score"`
}

type Hit struct {
	ID         string  `json:"id"`
	Category   string  `json:"category"`
	Similarity float32 `json:"similarity"`
}

type Response struct {
	Hits   []Hit   `json:"hits"`
	Scores []Score `json:"scores"`
}

type Server struct {
	embedClient       teipb.EmbedClient
	tokenizeClient    teipb.TokenizeClient
	pointsClient      qdrant.PointsClient
	scoresDB          *bolt.DB
	topK              int
	maxSequenceLength int
}

func NewServer(
	embedConn *grpc.ClientConn,
	qdrantConn *grpc.ClientConn,
	scoresDB *bolt.DB,
	topK int,
	maxSequenceLength int,
) *Server {
	return &Server{
		embedClient:       teipb.NewEmbedClient(embedConn),
		tokenizeClient:    teipb.NewTokenizeClient(embedConn),
		pointsClient:      qdrant.NewPointsClient(qdrantConn),
		scoresDB:          scoresDB,
		topK:              topK,
		maxSequenceLength: maxSequenceLength,
	}
}

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	// TODO (jeev): Switch to GRPC server
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse the payload from the request body
	payload := Request{TruncateStrategy: Middle}
	err = json.Unmarshal(body, &payload)
	if err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}
	if payload.Prompt == "" {
		http.Error(w, "Prompt is required", http.StatusBadRequest)
		return
	}

	res, err := s.query(r.Context(), &payload)
	if err != nil {
		http.Error(w, "Failed to retrieve scores", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (s *Server) sanitizePrompt(
	ctx context.Context,
	req *Request,
) (string, error) {
	encodeResp, err := s.tokenizeClient.Tokenize(ctx, &teipb.EncodeRequest{Inputs: req.Prompt})
	if err != nil {
		return "", fmt.Errorf("failed to tokenize prompt: %v", err)
	}
	numTokens := len(encodeResp.GetTokens())
	if numTokens <= s.maxSequenceLength {
		return req.Prompt, nil
	}

	switch req.TruncateStrategy {
	case Head:
		startToken := encodeResp.GetTokens()[numTokens-s.maxSequenceLength]
		return req.Prompt[*startToken.Start:], nil
	case Tail:
		endToken := encodeResp.GetTokens()[s.maxSequenceLength-1]
		return req.Prompt[:*endToken.Stop], nil
	case Middle:
		offset := s.maxSequenceLength / 2
		startTruncateToken := encodeResp.GetTokens()[offset]
		endTruncateToken := encodeResp.GetTokens()[numTokens+offset-s.maxSequenceLength-1]
		return req.Prompt[:*startTruncateToken.Start] + req.Prompt[*endTruncateToken.Stop:], nil
	case Ends:
		offset := (numTokens - s.maxSequenceLength) / 2
		startToken := encodeResp.GetTokens()[offset]
		endToken := encodeResp.GetTokens()[offset+s.maxSequenceLength-1]
		return req.Prompt[*startToken.Start:*endToken.Stop], nil
	}

	return "", fmt.Errorf("unsupported truncate strategy: %v", req.TruncateStrategy)
}

func (s *Server) query(
	ctx context.Context,
	req *Request,
) (*Response, error) {
	prompt, err := s.sanitizePrompt(ctx, req)
	if err != nil {
		log.Printf("Failed to sanitize prompt: %v", err)
		return nil, err
	}
	embedResp, err := s.embedClient.Embed(ctx, &teipb.EmbedRequest{Inputs: prompt, Truncate: true})
	if err != nil {
		log.Printf("Failed to compute embedding: %v", err)
		return nil, err
	}

	search, err := s.pointsClient.Search(ctx, &qdrant.SearchPoints{
		CollectionName: PointsCollection,
		Vector:         embedResp.GetEmbeddings(),
		Limit:          uint64(s.topK),
		WithVectors: &qdrant.WithVectorsSelector{
			SelectorOptions: &qdrant.WithVectorsSelector_Enable{Enable: false},
		},
		WithPayload: &qdrant.WithPayloadSelector{
			SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: false},
		},
	})
	if err != nil {
		log.Printf("Failed to search for nearest neighbors: %v", err)
		return nil, err
	}

	// Initialize response
	var res Response

	// Aggregate scores from nearest neighbors
	var weightSum float32
	scoresSum := make(map[string]float32)
	for _, pt := range search.GetResult() {
		uid := pt.GetId().GetUuid()
		weight := pt.GetScore()
		weightSum += weight
		// Lookup scores in KV store for given UID
		err = s.scoresDB.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(PointsCollection))
			v := b.Get([]byte(uid))
			if v == nil {
				return fmt.Errorf("could not find scores for nearest neighbor UID %s", uid)
			}
			var payload scorespb.Point
			err = proto.Unmarshal(v, &payload)
			if err != nil {
				return err
			}
			res.Hits = append(
				res.Hits,
				Hit{
					ID:         uid,
					Category:   payload.GetCategory(),
					Similarity: weight,
				},
			)
			for _, score := range payload.GetScores() {
				scoresSum[score.GetTarget()] += score.GetScore() * weight
			}
			return nil
		})
		if err != nil {
			log.Printf("Failed to retrieve scores for nearest neighbor UID %s: %v", uid, err)
			return nil, err
		}
	}
	// Normalize the accumulated scores by dividing by the sum of weighted distances
	for target, score := range scoresSum {
		normalizedScore := float32(math.Round(float64(score/weightSum)*100)) / 100
		res.Scores = append(
			res.Scores,
			Score{Target: target, Score: normalizedScore},
		)
	}
	return &res, nil
}

func (s *Server) ListenAndServe(bindAddr string) error {
	// TODO (jeev): Add prometheus metrics
	http.HandleFunc("/", s.handler)
	return http.ListenAndServe(bindAddr, nil)
}
