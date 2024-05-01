package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	Query            string           `json:"query"`
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
	DB                *bolt.DB
	topK              int
	maxSequenceLength int
}

func NewServer(
	embedConn *grpc.ClientConn,
	qdrantConn *grpc.ClientConn,
	DB *bolt.DB,
	topK int,
	maxSequenceLength int,
) *Server {
	return &Server{
		embedClient:       teipb.NewEmbedClient(embedConn),
		tokenizeClient:    teipb.NewTokenizeClient(embedConn),
		pointsClient:      qdrant.NewPointsClient(qdrantConn),
		DB:                DB,
		topK:              topK,
		maxSequenceLength: maxSequenceLength,
	}
}

func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	// TODO (jeev): Switch to GRPC server
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse the payload from the request body
	payload := Request{TruncateStrategy: Middle}
	err = json.Unmarshal(body, &payload)
	if err != nil {
		http.Error(w, "failed to parse request body", http.StatusBadRequest)
		return
	}
	if payload.Query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}

	res, err := s.query(r.Context(), &payload)
	if err != nil {
		http.Error(w, "failed to retrieve scores", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (s *Server) sanitizeQuery(
	ctx context.Context,
	req *Request,
) (string, error) {
	encodeResp, err := s.tokenizeClient.Tokenize(ctx, &teipb.EncodeRequest{Inputs: req.Query})
	if err != nil {
		return "", fmt.Errorf("failed to tokenize query: %v", err)
	}
	numTokens := len(encodeResp.GetTokens())
	if numTokens <= s.maxSequenceLength {
		return req.Query, nil
	}

	switch req.TruncateStrategy {
	case Head:
		startToken := encodeResp.GetTokens()[numTokens-s.maxSequenceLength]
		return req.Query[*startToken.Start:], nil
	case Tail:
		endToken := encodeResp.GetTokens()[s.maxSequenceLength-1]
		return req.Query[:*endToken.Stop], nil
	case Middle:
		offset := s.maxSequenceLength / 2
		startTruncateToken := encodeResp.GetTokens()[offset]
		endTruncateToken := encodeResp.GetTokens()[numTokens+offset-s.maxSequenceLength-1]
		return req.Query[:*startTruncateToken.Start] + req.Query[*endTruncateToken.Stop:], nil
	case Ends:
		offset := (numTokens - s.maxSequenceLength) / 2
		startToken := encodeResp.GetTokens()[offset]
		endToken := encodeResp.GetTokens()[offset+s.maxSequenceLength-1]
		return req.Query[*startToken.Start:*endToken.Stop], nil
	}

	return "", fmt.Errorf("unsupported truncate strategy: %v", req.TruncateStrategy)
}

func (s *Server) query(
	ctx context.Context,
	req *Request,
) (*Response, error) {
	query, err := s.sanitizeQuery(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to sanitize query: %v", err)
	}
	embedResp, err := s.embedClient.Embed(ctx, &teipb.EmbedRequest{Inputs: query, Truncate: true})
	if err != nil {
		return nil, fmt.Errorf("failed to compute embedding: %v", err)
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
		return nil, fmt.Errorf("failed to search for nearest neighbors: %v", err)
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
		// Lookup target scores in DB for given UID
		err = s.DB.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(PointsCollection))
			v := b.Get([]byte(uid))
			if v == nil {
				return fmt.Errorf("could not find targets for nearest neighbor UID %s", uid)
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
			return nil, fmt.Errorf(
				"failed to retrieve targets for nearest neighbor UID %s: %v",
				uid,
				err,
			)
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
