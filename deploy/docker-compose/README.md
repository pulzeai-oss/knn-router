# Docker Compose Deployment

As part of this quickstart, we present a dummy example that selects the most appropriate agent (i.e. `chitchat-agent` or `politics-agent`) for chit-chat and political queries respectively.
The example utterances are defined in [`./data/points.jsonl`](./data/points.jsonl), and the target agents and their corresponding score for each utterance are defined in [`./data/targets.jsonl`](./data/targets.jsonl).

Generate deployment artifacts:
```bash
../../scripts/gen-artifacts.sh --points-data-path data/points.jsonl --scores-data-path data/targets.jsonl --output-dir .
```

Start the services:
```bash
docker compose up -d --build
```

Get target scores for query:
```bash
curl -s 127.0.0.1:8888/ \
    -X POST \
    -d '{"query":"who are the candidates running for office?"}' \
    -H 'Content-Type: application/json' | jq .
```

Output:
```
{
  "hits": [
    {
      "id": "887cd819-15f5-4c27-a17b-532fec485271",
      "category": "politics",
      "similarity": 0.52368176
    },
    {
      "id": "3d0c0974-0d99-41fd-b7b1-4dfc9155d53f",
      "category": "politics",
      "similarity": 0.48429558
    },
    {
      "id": "442ae99e-c4d4-4316-9067-3b7232f23754",
      "category": "politics",
      "similarity": 0.47992623
    },
    {
      "id": "485d0e5b-599f-4844-99ab-d86f376c3214",
      "category": "politics",
      "similarity": 0.4750255
    },
    {
      "id": "b9c076c0-e96c-4208-88ec-761b325ca12e",
      "category": "politics",
      "similarity": 0.46624953
    },
    {
      "id": "9d7d61c1-a922-456c-913f-3c3a51beb8aa",
      "category": "chitchat",
      "similarity": 0.4377066
    },
    {
      "id": "8453a119-eff9-4473-bbb9-359adf9bf4c0",
      "category": "chitchat",
      "similarity": 0.43222725
    },
    {
      "id": "82dafcc6-b4eb-4ea1-967c-5d21453d881c",
      "category": "chitchat",
      "similarity": 0.41400117
    },
    {
      "id": "b9784f9c-ace9-4939-87d3-83e28dc44b5e",
      "category": "chitchat",
      "similarity": 0.39327258
    },
    {
      "id": "7f2ad3ce-4505-4d3b-89c1-178abc1aee80",
      "category": "chitchat",
      "similarity": 0.3586356
    }
  ],
  "scores": [
    {
      "target": "politics-agent",
      "score": 0.54
    },
    {
      "target": "chitchat-agent",
      "score": 0.46
    }
  ]
}
```
