package loader

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pulzeai-oss/knn-router/internal/scorespb"
	"github.com/pulzeai-oss/knn-router/internal/server"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"
)

type PointRow struct {
	PointUID string `json:"point_uid"`
	Category string `json:"category"`
}

type TargetScoreRow struct {
	PointUID string  `json:"point_uid"`
	Target   string  `json:"target"`
	Score    float32 `json:"score"`
}

type Loader struct {
	points map[string]*scorespb.Point
}

func NewLoader() *Loader {
	return &Loader{points: make(map[string]*scorespb.Point)}
}

func (l *Loader) LoadPoints(pointsDataPath string) error {
	// Read in the JSONL-formatted dataset source
	dataFile, err := os.Open(pointsDataPath)
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
		var row PointRow
		err := json.Unmarshal(line, &row)
		if err != nil {
			return err
		}
		l.points[row.PointUID] = &scorespb.Point{Category: row.Category}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func (l *Loader) LoadScores(scoresDataPath string) error {
	// Read in the JSONL-formatted dataset source
	dataFile, err := os.Open(scoresDataPath)
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
		var row TargetScoreRow
		err := json.Unmarshal(line, &row)
		if err != nil {
			return err
		}

		p, exists := l.points[row.PointUID]
		if !exists {
			return fmt.Errorf("point UID '%s' not found", row.PointUID)
		}
		p.Scores = append(
			p.Scores,
			&scorespb.Score{Target: row.Target, Score: row.Score},
		)
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func (l *Loader) SaveScores(scoresDBPath string) error {
	// Write to the scores database
	scoresDB, err := bolt.Open(scoresDBPath, 0600, nil)
	if err != nil {
		return err
	}
	defer scoresDB.Close()
	return scoresDB.Update(func(tx *bolt.Tx) error {
		for pointUID, point := range l.points {
			b, err := tx.CreateBucketIfNotExists([]byte(server.PointsCollection))
			if err != nil {
				return err
			}
			v, err := proto.Marshal(point)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(pointUID), v); err != nil {
				return err
			}
		}

		return nil
	})
}
