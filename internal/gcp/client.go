package gcp

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

// Client wraps GCP service clients.
type Client struct {
	Storage  *storage.Client
	SQLAdmin *sqladmin.Service
}

// NewClient creates GCP clients using Application Default Credentials (workload identity).
func NewClient(ctx context.Context) (*Client, error) {
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating storage client: %w", err)
	}

	sqlAdminService, err := sqladmin.NewService(ctx)
	if err != nil {
		_ = storageClient.Close()
		return nil, fmt.Errorf("creating sqladmin client: %w", err)
	}

	return &Client{
		Storage:  storageClient,
		SQLAdmin: sqlAdminService,
	}, nil
}

// Close cleans up client resources.
func (c *Client) Close() error {
	if c.Storage != nil {
		return c.Storage.Close()
	}
	return nil
}
