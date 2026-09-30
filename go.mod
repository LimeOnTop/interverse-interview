module github.com/LimeOnTop/interverse-interview

go 1.25.0

require (
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.12.3
	google.golang.org/grpc v1.76.0
)

require github.com/LimeOnTop/interverse-contracts v0.0.0-20260805234728-a1c1a5bacbb9

require (
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250804133106-a7a43d27e69b // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

replace github.com/LimeOnTop/interverse-contracts => ../interverse-contracts
