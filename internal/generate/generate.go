package generate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/pulzeai-oss/knn-router/internal/scorespb"
	"github.com/pulzeai-oss/knn-router/internal/server"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"
)

type Example struct {
	UID      string  `json:"uid"`
	Category string  `json:"category"`
	Target   string  `json:"target"`
	Score    float32 `json:"score"`
}

func LoadDB(dataPath string, scoresDBPath string) error {
	pointsMap := make(map[string]*scorespb.Point)

	// Read in the JSONL-formatted dataset source
	dataFile, err := os.Open(dataPath)
	if err != nil {
		return err
	}
	defer dataFile.Close()
	scanner := bufio.NewScanner(dataFile)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var example Example
		err := json.Unmarshal(line, &example)
		if err != nil {
			return err
		}

		p, exists := pointsMap[example.UID]
		if !exists {
			p = &scorespb.Point{Category: example.Category}
			pointsMap[example.UID] = p
		}
		if p.Category != example.Category {
			return fmt.Errorf(
				"UID '%s' has conflicting categories: Expected '%s', but got '%s'",
				example.UID,
				p.Category,
				example.Category,
			)
		}
		p.Scores = append(
			p.Scores,
			&scorespb.Score{Target: example.Target, Score: example.Score},
		)
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	// Write to the scores database
	scoresDB, err := bolt.Open(scoresDBPath, 0600, nil)
	if err != nil {
		log.Fatalf("Failed to open scores database: %v", err)
	}
	defer scoresDB.Close()
	return scoresDB.Update(func(tx *bolt.Tx) error {
		for promptUID, point := range pointsMap {
			b, err := tx.CreateBucketIfNotExists([]byte(server.PointsCollection))
			if err != nil {
				return err
			}
			v, err := proto.Marshal(point)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(promptUID), v); err != nil {
				return err
			}
		}

		return nil
	})
}
