module github.com/gnexlayer/demo-service

go 1.22

require (
	github.com/gorilla/websocket v1.5.3
	google.golang.org/grpc v1.64.0
)

// After cloning: run `go mod tidy` to generate go.sum, then commit it.
// The Dockerfile runs `go mod tidy` automatically on first build.
