module github.com/pulzeai-oss/knn-router

go 1.22.0

require (
	github.com/pulzeai-oss/knn-router/internal/scorespb v0.0.0-00010101000000-000000000000
	github.com/pulzeai-oss/knn-router/internal/teipb v0.0.0-00010101000000-000000000000
	github.com/qdrant/go-client v1.7.0
	github.com/spf13/cobra v1.8.0
	go.etcd.io/bbolt v1.3.9
	google.golang.org/grpc v1.61.1
	google.golang.org/protobuf v1.32.0
)

require (
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/net v0.18.0 // indirect
	golang.org/x/sys v0.14.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231106174013-bbf56f31fb17 // indirect
)

replace github.com/pulzeai-oss/knn-router/internal/scorespb => ./internal/scorespb

replace github.com/pulzeai-oss/knn-router/internal/teipb => ./internal/teipb
