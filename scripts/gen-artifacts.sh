#!/usr/bin/env bash

set -euxo pipefail

ROOT_DIR="$(realpath "$(dirname "$0")/..")"

_cleanup() {
    [[ -z "${TMPDIR:-}" ]] || rm -rf ${TMPDIR}
    [[ -z "${QDRANT_CNT:-}" ]] || docker kill ${QDRANT_CNT}
}
trap _cleanup EXIT

# Parse command line arguments
DISTANCE_METRIC="Cosine"
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        --points-data-path)
        POINTS_DATA_PATH="$2"
        shift
        ;;
        --scores-data-path)
        SCORES_DATA_PATH="$2"
        shift
        ;;
        --distance-metric)
        DISTANCE_METRIC="$2"
        shift
        ;;
        --output-dir)
        OUTPUT_DIR="$2"
        shift
        ;;
        *)
        # Unknown option
        echo "Unknown option: $1"
        exit 1
        ;;
    esac
    shift
done

# Create the output directory
mkdir -p ${OUTPUT_DIR}

# Generate Bolt DB of targets/scores
go run ${ROOT_DIR}/main.go load --points-data-path ${POINTS_DATA_PATH} --scores-data-path ${SCORES_DATA_PATH} --db-path ${OUTPUT_DIR}/scores.db

# Generate embeddings.snapshot
TMPDIR=$(mktemp -d)
echo ${TMPDIR}
QDRANT_CNT=$(docker run -d -e QDRANT__STORAGE__SNAPSHOTS_PATH=/tmp/snapshots -it -p 6335:6333 --rm -u "$(id -u)" -v ${TMPDIR}:/tmp/snapshots ghcr.io/qdrant/qdrant/qdrant:v1.9.0-unprivileged ./qdrant)

# Wait for Qdrant
timeout 1m bash -c 'until curl -s http://localhost:6335/readyz; do sleep 1; done'

# Create a collection
curl --fail -X PUT http://localhost:6335/collections/main \
  -H 'Content-Type: application/json' \
  --data-raw "{
    \"vectors\": {
      \"size\": $(head -n 1 ${POINTS_DATA_PATH} | jq '.embedding | length'),
      \"distance\": \"${DISTANCE_METRIC}\"
    } 
  }"

# Batch upsert points
jq -c '{id: .point_uid, vector: .embedding, payload: {}}' ${POINTS_DATA_PATH} | split -l 500 --filter="cat > ${TMPDIR}/points_\$FILE.jsonl"
for f in ${TMPDIR}/points_*.jsonl; do
    jq -cs '{operations: [{upsert: {points: .}}]}' ${f} > ${f}.payload
    curl -X POST http://localhost:6335/collections/main/points/batch \
    -H 'Content-Type: application/json' \
    --data-binary "@${f}.payload"
done

# Save snapshot
curl --fail -X POST http://localhost:6335/collections/main/snapshots \
    -H 'Content-Type: application/json'

# Move snapshot to the output directory
mv ${TMPDIR}/main/main-*.snapshot ${OUTPUT_DIR}/embeddings.snapshot
