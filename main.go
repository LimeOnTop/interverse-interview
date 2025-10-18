package main

import (
	"log"
	"net"

	pb "github.com/inter-verse/services/interview-service/gen"
	"github.com/inter-verse/services/interview-service/internal/config"
	"github.com/inter-verse/services/interview-service/internal/database"
	"github.com/inter-verse/services/interview-service/internal/handler"
	"github.com/inter-verse/services/interview-service/internal/repository"
	"github.com/inter-verse/services/interview-service/internal/service"
	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.NewConnection(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	interviewRepo := repository.NewInterviewRepository(db)

	// Initialize service
	interviewService := service.NewInterviewService(*interviewRepo, cfg.UserServiceURL, cfg.QuestionServiceURL)

	// Initialize gRPC server
	lis, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterInterviewServiceServer(grpcServer, handler.NewInterviewHandler(interviewService))

	log.Printf("Interview Service starting on port %s", cfg.Port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
