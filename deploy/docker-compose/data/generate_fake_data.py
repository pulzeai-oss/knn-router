import uuid

import pandas as pd
from sentence_transformers import SentenceTransformer


def _fake_points():
    embedding_model = SentenceTransformer("BAAI/bge-small-en-v1.5")
    df = pd.DataFrame(
        [
            *[
                {
                    "point_uid": str(uuid.uuid4()),
                    "category": "politics",
                    "utterance": utterance,
                }
                for utterance in [
                    "isn't politics the best thing ever",
                    "why don't you tell me about your political opinions",
                    "don't you just love the president",
                    "they're going to destroy this country!",
                    "they will save the country!",
                ]
            ],
            *[
                {
                    "point_uid": str(uuid.uuid4()),
                    "category": "chitchat",
                    "utterance": utterance,
                }
                for utterance in [
                    "how's the weather today?",
                    "how are things going?",
                    "lovely weather today",
                    "the weather is horrendous",
                    "let's go to the chippy",
                ]
            ],
        ]
    )
    df["embedding"] = embedding_model.encode(df["utterance"].tolist()).tolist()
    return df


def _fake_targets(points):
    return pd.DataFrame(
        [
            {
                "point_uid": point["point_uid"],
                "target": "politics-agent"
                if point["category"] == "politics"
                else "chitchat-agent",
                "score": 1,
            }
            for _, point in points.iterrows()
        ]
    )


if __name__ == "__main__":
    points = _fake_points()
    with open("points.jsonl", "w") as f:
        print(points.to_json(orient="records", lines=True), file=f)
    targets = _fake_targets(points)
    with open("targets.jsonl", "w") as f:
        print(targets.to_json(orient="records", lines=True), file=f)
