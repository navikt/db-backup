package gcp

import (
	"context"
	"fmt"
	"log/slog"

	"cloud.google.com/go/iam"
)

// GrantBucketObjectCreator grants roles/storage.objectCreator to a service account on the bucket.
func (c *Client) GrantBucketObjectCreator(ctx context.Context, bucket, serviceAccountEmail string) error {
	bucketHandle := c.Storage.Bucket(bucket)
	policy, err := bucketHandle.IAM().Policy(ctx)
	if err != nil {
		return fmt.Errorf("getting bucket IAM policy: %w", err)
	}

	member := fmt.Sprintf("serviceAccount:%s", serviceAccountEmail)
	role := iam.RoleName("roles/storage.objectCreator")

	// Check if binding already exists
	for _, m := range policy.Members(role) {
		if m == member {
			slog.Debug("IAM binding already exists", "bucket", bucket, "member", member)
			return nil
		}
	}

	policy.Add(member, role)
	if err := bucketHandle.IAM().SetPolicy(ctx, policy); err != nil {
		return fmt.Errorf("setting bucket IAM policy: %w", err)
	}

	slog.Info("granted objectCreator", "bucket", bucket, "member", member)
	return nil
}
