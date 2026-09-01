package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/LimeOnTop/interverse-interview/cmd/config"
	"github.com/LimeOnTop/interverse-interview/internal/client"
	"github.com/LimeOnTop/interverse-interview/internal/controller"
	"github.com/LimeOnTop/interverse-interview/internal/repository"
	"github.com/LimeOnTop/interverse-interview/internal/service"
	pb "github.com/LimeOnTop/interverse-contracts/interview/gen"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

const shutdownTimeout = 15 * time.Second

func main() {
	cfg := config.Load()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		panic("open database: " + err.Error())
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(30)
	db.SetConnMaxIdleTime(30 * time.Minute)

	defer db.Close()

	interviewRepository := repository.NewInterviewRepository(db)

	questionClient, err := client.NewQuestionClient(cfg.QuestionServiceURL)
	if err != nil {
		panic("question client: " + err.Error())
	}

	interviewService := service.NewInterviewService(interviewRepository, questionClient)
	interviewController := controller.NewInterviewController(interviewService)

	lis, err := net.Listen("tcp", net.JoinHostPort("", cfg.Port))
	if err != nil {
		panic("listen: " + err.Error())
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(recoveryUnary()),
	)

	pb.RegisterInterviewServiceServer(grpcServer, interviewController)

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("interview service listening on %s", lis.Addr())
		errCh <- grpcServer.Serve(lis)
	}()

	defer close(errCh)

	waitForShutdown(grpcServer, healthServer, errCh)
}

func recoveryUnary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf(
					"panic recovered: method=%s panic=%v\n%s",
					info.FullMethod,
					r,
					debug.Stack(),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

func waitForShutdown(grpcServer *grpc.Server, healthServer *health.Server, errCh <-chan error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("shutdown signal received: %s", sig)
	case err := <-errCh:
		if err != nil && err != grpc.ErrServerStopped {
			panic("grpc serve: " + err.Error())
		}
		return
	}

	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		log.Print("grpc server stopped")
	case <-time.After(shutdownTimeout):
		log.Print("shutdown timed out, forcing stop")
		grpcServer.Stop()
	}
}
