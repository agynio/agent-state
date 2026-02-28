package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	agentstatev1 "github.com/agynio/agent-state/gen/go/agynio/api/agent_state/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	"github.com/agynio/agent-state/internal/config"
	"github.com/agynio/agent-state/internal/db"
	"github.com/agynio/agent-state/internal/server"
	"github.com/agynio/agent-state/internal/state"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("agent-state-service: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("create connection pool: %w", err)
	}
	defer pool.Close()

	if err := db.ApplyMigrations(ctx, pool); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	grpcServer := grpc.NewServer()
	agentstatev1.RegisterAgentStateServiceServer(grpcServer, server.New(state.NewStore(pool)))

	lis, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.GRPCAddress, err)
	}

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	log.Printf("AgentStateService listening on %s", cfg.GRPCAddress)

	if err := grpcServer.Serve(lis); err != nil {
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
