package gcp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

const operationPollTimeout = 20 * time.Hour

// ExportDatabase starts an async SQL export to a GCS URI.
func (c *Client) ExportDatabase(ctx context.Context, projectID, instance, database, gcsURI string) (string, error) {
	req := &sqladmin.InstancesExportRequest{
		ExportContext: &sqladmin.ExportContext{
			FileType:  "SQL",
			Uri:       gcsURI,
			Databases: []string{database},
			Offload:   true,
		},
	}

	op, err := c.SQLAdmin.Instances.Export(projectID, instance, req).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("starting export for %s/%s: %w", projectID, instance, err)
	}

	slog.Info("export started", "instance", instance, "database", database, "operation", op.Name)
	return op.Name, nil
}

// WaitForOperation polls a Cloud SQL operation until it completes or times out.
func (c *Client) WaitForOperation(ctx context.Context, projectID, operationName string) error {
	deadline := time.Now().Add(operationPollTimeout)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("operation %s timed out after %v", operationName, operationPollTimeout)
		}

		op, err := c.SQLAdmin.Operations.Get(projectID, operationName).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("getting operation %s: %w", operationName, err)
		}

		if op.Status == "DONE" {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				return fmt.Errorf("operation %s failed: %s", operationName, op.Error.Errors[0].Message)
			}
			slog.Info("operation completed", "operation", operationName)
			return nil
		}

		slog.Debug("operation pending", "operation", operationName, "status", op.Status)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(30 * time.Second):
		}
	}
}

// ListPendingOperations returns pending (non-DONE) operations for an instance.
func (c *Client) ListPendingOperations(ctx context.Context, projectID, instance string) ([]string, error) {
	resp, err := c.SQLAdmin.Operations.List(projectID).Instance(instance).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("listing operations for %s: %w", instance, err)
	}

	var pending []string
	for _, op := range resp.Items {
		if op.Status != "DONE" {
			pending = append(pending, op.Name)
		}
	}

	return pending, nil
}
