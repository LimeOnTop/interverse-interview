module github.com/inter-verse/services/interview-service

go 1.24

require (
	github.com/google/uuid v1.6.0
	github.com/inter-verse/services/proto/gen v0.0.0-00010101000000-000000000000
	github.com/inter-verse/services/interview-service/gen v0.0.0-00010101000000-000000000000
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.10.9
	google.golang.org/grpc v1.76.0
)

replace github.com/inter-verse/services/proto/gen => ../proto/gen
replace github.com/inter-verse/services/interview-service/gen => ./gen
