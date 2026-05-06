package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nais/db-backup/internal/backup"
	"github.com/nais/db-backup/internal/gcp"
	"github.com/nais/db-backup/internal/k8s"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	bucketName := os.Getenv("BUCKET_NAME")
	if bucketName == "" {
		slog.Error("BUCKET_NAME environment variable not set")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Overall timeout — slightly under the Naisjob activeDeadlineSeconds (72000s = 20h)
	// so we log a clean error before k8s force-kills the pod.
	ctx, timeoutCancel := context.WithTimeout(ctx, 19*time.Hour+50*time.Minute)
	defer timeoutCancel()

	k8sClient, err := k8s.NewClient()
	if err != nil {
		slog.Error("failed to create kubernetes client", "error", err)
		os.Exit(1)
	}

	gcpClient, err := gcp.NewClient(ctx)
	if err != nil {
		slog.Error("failed to create GCP client", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := gcpClient.Close(); err != nil {
			slog.Error("failed to close GCP client", "error", err)
		}
	}()

	runner := backup.NewRunner(k8sClient, gcpClient, bucketName)
	if err := runner.Run(ctx); err != nil {
		slog.Error("backup failed", "error", err)
		os.Exit(1)
	}

	slog.Info("backup completed successfully")
}
