package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	composeProject = "agent-state-e2e"
	dbURL          = "postgres://agentstate:agentstate@localhost:55432/agentstate?sslmode=disable"
)

var (
	composeFile    = mustAbs(filepath.Join("..", "..", "docker-compose.yml"))
	composeCommand string
	composePrefix  []string
)

func TestMain(m *testing.M) {
	if err := resolveCompose(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve docker-compose: %v\n", err)
		os.Exit(1)
	}
	if err := dockerComposeUp(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start docker-compose: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	if err := dockerComposeDown(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to stop docker-compose: %v\n", err)
	}
	os.Exit(code)
}

func resolveCompose() error {
	if err := exec.Command("docker", "compose", "version").Run(); err == nil {
		composeCommand = "docker"
		composePrefix = []string{"compose"}
		return nil
	}
	if path, err := exec.LookPath("docker-compose"); err == nil {
		composeCommand = path
		composePrefix = nil
		return nil
	}
	binPath, err := downloadComposeBinary()
	if err != nil {
		return err
	}
	composeCommand = binPath
	composePrefix = nil
	return nil
}

func downloadComposeBinary() (string, error) {
	url := "https://github.com/docker/compose/releases/download/v2.27.0/docker-compose-linux-x86_64"
	binDir := filepath.Join(os.TempDir(), "agent-state-compose")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(binDir, "docker-compose")
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status downloading docker-compose: %s", resp.Status)
	}
	file, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", err
	}
	if err := file.Chmod(0o755); err != nil {
		return "", err
	}
	return dest, nil
}

func dockerComposeUp() error {
	_ = runCompose("down", "-v")
	if err := runCompose("up", "-d"); err != nil {
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

func dockerComposeDown() error {
	return runCompose("down", "-v")
}

func runCompose(args ...string) error {
	base := append([]string(nil), composePrefix...)
	base = append(base, "-f", composeFile, "-p", composeProject)
	return runCommand(composeCommand, append(base, args...)...)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return abs
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
