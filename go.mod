module singboxwin.local/client

go 1.27.3

require github.com/egoist/mygo v0.0.0

require (
	github.com/Microsoft/go-winio v0.6.2
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	golang.org/x/image v0.46.0 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.1
	google.golang.org/protobuf v1.36.11
)

replace github.com/egoist/mygo => ./third_party/mygo
