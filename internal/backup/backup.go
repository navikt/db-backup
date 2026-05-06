package backup

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nais/db-backup/internal/gcp"
	"github.com/nais/db-backup/internal/k8s"
)

// K8sClient defines the Kubernetes operations needed by the backup runner.
type K8sClient interface {
	ListSQLDatabases(ctx context.Context) ([]k8s.SQLDatabase, error)
	GetNamespaceProjectID(ctx context.Context, namespace string) (string, error)
	SQLInstanceExists(ctx context.Context, namespace, name string) bool
	GetSQLInstance(ctx context.Context, namespace, name string) (*k8s.SQLInstance, error)
}

// GCPClient defines the GCP operations needed by the backup runner.
type GCPClient interface {
	GrantBucketObjectCreator(ctx context.Context, bucket, serviceAccountEmail string) error
	ObjectExists(ctx context.Context, bucket, object string) (bool, error)
	ExportDatabase(ctx context.Context, projectID, instance, database, gcsURI string) (string, error)
	WaitForOperation(ctx context.Context, projectID, operationName string) error
}

// pendingOp tracks an async export operation.
type pendingOp struct {
	ProjectID     string
	OperationName string
	Instance      string
}

// Runner orchestrates the backup process.
type Runner struct {
	k8s        K8sClient
	gcp        GCPClient
	bucketName string
}

// NewRunner creates a new backup runner.
func NewRunner(k8sClient *k8s.Client, gcpClient *gcp.Client, bucketName string) *Runner {
	return &Runner{
		k8s:        k8sClient,
		gcp:        gcpClient,
		bucketName: bucketName,
	}
}

// NewRunnerWithInterfaces creates a runner from interface implementations (useful for testing).
func NewRunnerWithInterfaces(k8sClient K8sClient, gcpClient GCPClient, bucketName string) *Runner {
	return &Runner{
		k8s:        k8sClient,
		gcp:        gcpClient,
		bucketName: bucketName,
	}
}

// Run executes the full backup workflow.
func (r *Runner) Run(ctx context.Context) error {
	databases, err := r.k8s.ListSQLDatabases(ctx)
	if err != nil {
		return fmt.Errorf("listing databases: %w", err)
	}

	slog.Info("found databases", "count", len(databases))

	// Group databases by namespace
	nsByDBs := make(map[string][]k8s.SQLDatabase)
	for _, db := range databases {
		nsByDBs[db.Namespace] = append(nsByDBs[db.Namespace], db)
	}

	var ops []pendingOp
	datePrefix := time.Now().Format("20060102")

	// Phase 1: Start all exports
	for namespace, dbs := range nsByDBs {
		slog.Info("processing namespace", "namespace", namespace, "databases", len(dbs))

		projectID, err := r.k8s.GetNamespaceProjectID(ctx, namespace)
		if err != nil {
			slog.Error("failed to get project ID", "namespace", namespace, "error", err)
			continue
		}

		for _, db := range dbs {
			if !r.k8s.SQLInstanceExists(ctx, namespace, db.InstanceRef) {
				slog.Warn("instance referenced in database does not exist, skipping",
					"database", db.Name, "instance", db.InstanceRef, "namespace", namespace)
				continue
			}

			instance, err := r.k8s.GetSQLInstance(ctx, namespace, db.InstanceRef)
			if err != nil {
				slog.Error("failed to get sqlinstance", "instance", db.InstanceRef, "error", err)
				continue
			}

			// Grant IAM permissions
			if instance.ServiceAccountEmailAddress != "" {
				if err := r.gcp.GrantBucketObjectCreator(ctx, r.bucketName, instance.ServiceAccountEmailAddress); err != nil {
					slog.Error("failed to grant IAM", "instance", db.InstanceRef, "error", err)
					continue
				}
			}

			// Check if backup already exists
			objectPath := fmt.Sprintf("%s/%s_%s.gz", namespace, datePrefix, db.InstanceRef)
			exists, err := r.gcp.ObjectExists(ctx, r.bucketName, objectPath)
			if err != nil {
				slog.Error("failed to check object existence", "path", objectPath, "error", err)
				continue
			}
			if exists {
				slog.Info("backup already exists, skipping", "path", objectPath)
				continue
			}

			// Start export
			gcsURI := fmt.Sprintf("gs://%s/%s", r.bucketName, objectPath)
			opName, err := r.gcp.ExportDatabase(ctx, projectID, db.InstanceRef, db.ResourceID, gcsURI)
			if err != nil {
				slog.Error("failed to start export", "instance", db.InstanceRef, "database", db.ResourceID, "error", err)
				continue
			}

			ops = append(ops, pendingOp{
				ProjectID:     projectID,
				OperationName: opName,
				Instance:      db.InstanceRef,
			})
		}
	}

	// Phase 2: Wait for all operations to complete
	slog.Info("waiting for operations", "count", len(ops))
	var errs []error
	for _, op := range ops {
		if err := r.gcp.WaitForOperation(ctx, op.ProjectID, op.OperationName); err != nil {
			slog.Error("operation failed", "instance", op.Instance, "operation", op.OperationName, "error", err)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d operations failed", len(errs))
	}

	return nil
}
