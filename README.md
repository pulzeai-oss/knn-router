# KNN Router

A minimal server for generating a ranked (weighted average) list of targets, for a query, based on its k-nearest semantic neighbors. Written in Go.

Works with:

- Embeddings: [HuggingFace Text Embeddings Inference](https://github.com/huggingface/text-embeddings-inference)
- Vector Store: [Qdrant](https://github.com/qdrant/qdrant)
- Database: [Bolt](https://github.com/etcd-io/bbolt)
