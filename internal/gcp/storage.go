package gcp

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/storage"
)

// ObjectExists checks if an object exists in GCS.
func (c *Client) ObjectExists(ctx context.Context, bucket, object string) (bool, error) {
	_, err := c.Storage.Bucket(bucket).Object(object).Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("checking object %s/%s: %w", bucket, object, err)
	}
	return true, nil
}
