package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	postgresContainer = "agent-state-e2e-postgres"
	postgresImage     = "public.ecr.aws/docker/library/postgres:16-alpine"
	dbURL             = "postgres://agentstate:agentstate@localhost:55432/agentstate?sslmode=disable"
)

func TestMain(m *testing.M) {
	if err := ensureDocker(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve docker: %v\n", err)
		os.Exit(1)
	}
	if err := startPostgres(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	if err := stopPostgres(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to stop postgres: %v\n", err)
	}
	os.Exit(code)
}

func ensureDocker() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return err
	}
	return runCommand("docker", "version", "--format", "{{.Server.Version}}")
}

func startPostgres() error {
	_ = runCommand("docker", "rm", "-f", postgresContainer)
	if err := runCommand(
		"docker",
		"run",
		"--name",
		postgresContainer,
		"-e",
		"POSTGRES_USER=agentstate",
		"-e",
		"POSTGRES_PASSWORD=agentstate",
		"-e",
		"POSTGRES_DB=agentstate",
		"-p",
		"55432:5432",
		"-d",
		postgresImage,
	); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	for {
		if ctx.Err() != nil {
			return fmt.Errorf("timeout waiting for postgres")
		}
		if err := pingDatabase(ctx); err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		return nil
	}
}

func stopPostgres() error {
	return runCommand("docker", "rm", "-f", postgresContainer)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func pingDatabase(ctx context.Context) error {
	connCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	conn, err := pgx.Connect(connCtx, dbURL)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	return nil
}
