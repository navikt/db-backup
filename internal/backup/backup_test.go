package backup

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nais/db-backup/internal/k8s"
)

// mockK8s implements K8sClient for testing.
type mockK8s struct {
	databases  []k8s.SQLDatabase
	projectIDs map[string]string           // namespace -> projectID
	instances  map[string]*k8s.SQLInstance // "namespace/name" -> instance
}

func (m *mockK8s) ListSQLDatabases(_ context.Context) ([]k8s.SQLDatabase, error) {
	return m.databases, nil
}

func (m *mockK8s) GetNamespaceProjectID(_ context.Context, namespace string) (string, error) {
	id, ok := m.projectIDs[namespace]
	if !ok {
		return "", fmt.Errorf("namespace %s not found", namespace)
	}
	return id, nil
}

func (m *mockK8s) GetSQLInstance(_ context.Context, namespace, name string) (*k8s.SQLInstance, error) {
	inst, ok := m.instances[namespace+"/"+name]
	if !ok {
		return nil, nil
	}
	return inst, nil
}

// mockGCP implements GCPClient for testing.
type mockGCP struct {
	existingObjects map[string]bool // "bucket/object" -> exists
	grantCalls      []string        // service account emails granted
	exportCalls     []exportCall    // exports started
	waitCalls       []string        // operation names waited on
	exportErr       error
	waitErr         error
}

type exportCall struct {
	ProjectID string
	Instance  string
	Database  string
	URI       string
}

func (m *mockGCP) GrantBucketObjectCreator(_ context.Context, bucket, sa string) error {
	m.grantCalls = append(m.grantCalls, sa)
	return nil
}

func (m *mockGCP) ObjectExists(_ context.Context, bucket, object string) (bool, error) {
	key := bucket + "/" + object
	return m.existingObjects[key], nil
}

func (m *mockGCP) ExportDatabase(_ context.Context, projectID, instance, database, gcsURI string) (string, error) {
	if m.exportErr != nil {
		return "", m.exportErr
	}
	m.exportCalls = append(m.exportCalls, exportCall{
		ProjectID: projectID,
		Instance:  instance,
		Database:  database,
		URI:       gcsURI,
	})
	return fmt.Sprintf("op-%s-%s", projectID, instance), nil
}

func (m *mockGCP) WaitForOperation(_ context.Context, projectID, operationName string) error {
	m.waitCalls = append(m.waitCalls, operationName)
	return m.waitErr
}

func TestRunnerRun_ExportsNewBackup(t *testing.T) {
	k8sMock := &mockK8s{
		databases: []k8s.SQLDatabase{
			{Name: "my-db", Namespace: "team-a", InstanceRef: "pg-1", ResourceID: "mydb"},
		},
		projectIDs: map[string]string{"team-a": "project-a"},
		instances: map[string]*k8s.SQLInstance{
			"team-a/pg-1": {
				Name:                       "pg-1",
				Namespace:                  "team-a",
				ServiceAccountEmailAddress: "sql@gcp.iam.gserviceaccount.com",
			},
		},
	}

	gcpMock := &mockGCP{
		existingObjects: map[string]bool{}, // no existing backups
	}

	runner := NewRunnerWithInterfaces(k8sMock, gcpMock, "my-backup-bucket")
	ctx := context.Background()

	err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify IAM grant was called
	if len(gcpMock.grantCalls) != 1 {
		t.Fatalf("expected 1 grant call, got %d", len(gcpMock.grantCalls))
	}
	if gcpMock.grantCalls[0] != "sql@gcp.iam.gserviceaccount.com" {
		t.Errorf("granted wrong SA: %s", gcpMock.grantCalls[0])
	}

	// Verify export was started
	if len(gcpMock.exportCalls) != 1 {
		t.Fatalf("expected 1 export call, got %d", len(gcpMock.exportCalls))
	}
	export := gcpMock.exportCalls[0]
	if export.ProjectID != "project-a" {
		t.Errorf("wrong projectID: %s", export.ProjectID)
	}
	if export.Instance != "pg-1" {
		t.Errorf("wrong instance: %s", export.Instance)
	}
	if export.Database != "mydb" {
		t.Errorf("wrong database: %s", export.Database)
	}
	datePrefix := time.Now().Format("20060102")
	wantURI := fmt.Sprintf("gs://my-backup-bucket/team-a/%s_pg-1.gz", datePrefix)
	if export.URI != wantURI {
		t.Errorf("wrong URI: got %s, want %s", export.URI, wantURI)
	}

	// Verify wait was called
	if len(gcpMock.waitCalls) != 1 {
		t.Fatalf("expected 1 wait call, got %d", len(gcpMock.waitCalls))
	}
}

func TestRunnerRun_SkipsExistingBackup(t *testing.T) {
	datePrefix := time.Now().Format("20060102")

	k8sMock := &mockK8s{
		databases: []k8s.SQLDatabase{
			{Name: "my-db", Namespace: "team-a", InstanceRef: "pg-1", ResourceID: "mydb"},
		},
		projectIDs: map[string]string{"team-a": "project-a"},
		instances: map[string]*k8s.SQLInstance{
			"team-a/pg-1": {
				Name:                       "pg-1",
				Namespace:                  "team-a",
				ServiceAccountEmailAddress: "sql@gcp.iam.gserviceaccount.com",
			},
		},
	}

	gcpMock := &mockGCP{
		existingObjects: map[string]bool{
			fmt.Sprintf("my-backup-bucket/team-a/%s_pg-1.gz", datePrefix): true,
		},
	}

	runner := NewRunnerWithInterfaces(k8sMock, gcpMock, "my-backup-bucket")
	ctx := context.Background()

	err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No exports should have been started
	if len(gcpMock.exportCalls) != 0 {
		t.Errorf("expected 0 export calls, got %d", len(gcpMock.exportCalls))
	}
	if len(gcpMock.waitCalls) != 0 {
		t.Errorf("expected 0 wait calls, got %d", len(gcpMock.waitCalls))
	}
}

func TestRunnerRun_SkipsMissingInstance(t *testing.T) {
	k8sMock := &mockK8s{
		databases: []k8s.SQLDatabase{
			{Name: "my-db", Namespace: "team-a", InstanceRef: "nonexistent", ResourceID: "mydb"},
		},
		projectIDs: map[string]string{"team-a": "project-a"},
		instances:  map[string]*k8s.SQLInstance{}, // no instances
	}

	gcpMock := &mockGCP{
		existingObjects: map[string]bool{},
	}

	runner := NewRunnerWithInterfaces(k8sMock, gcpMock, "my-backup-bucket")
	ctx := context.Background()

	err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gcpMock.exportCalls) != 0 {
		t.Errorf("expected 0 export calls for missing instance, got %d", len(gcpMock.exportCalls))
	}
}

func TestRunnerRun_ReportsOperationFailure(t *testing.T) {
	k8sMock := &mockK8s{
		databases: []k8s.SQLDatabase{
			{Name: "my-db", Namespace: "team-a", InstanceRef: "pg-1", ResourceID: "mydb"},
		},
		projectIDs: map[string]string{"team-a": "project-a"},
		instances: map[string]*k8s.SQLInstance{
			"team-a/pg-1": {
				Name:                       "pg-1",
				Namespace:                  "team-a",
				ServiceAccountEmailAddress: "sql@gcp.iam.gserviceaccount.com",
			},
		},
	}

	gcpMock := &mockGCP{
		existingObjects: map[string]bool{},
		waitErr:         errors.New("operation timed out"),
	}

	runner := NewRunnerWithInterfaces(k8sMock, gcpMock, "my-backup-bucket")
	ctx := context.Background()

	err := runner.Run(ctx)
	if err == nil {
		t.Fatal("expected error when operation fails, got nil")
	}
	if err.Error() != "1 operations failed" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestRunnerRun_MultipleNamespaces(t *testing.T) {
	k8sMock := &mockK8s{
		databases: []k8s.SQLDatabase{
			{Name: "db-a", Namespace: "team-a", InstanceRef: "pg-a", ResourceID: "dba"},
			{Name: "db-b", Namespace: "team-b", InstanceRef: "pg-b", ResourceID: "dbb"},
		},
		projectIDs: map[string]string{
			"team-a": "project-a",
			"team-b": "project-b",
		},
		instances: map[string]*k8s.SQLInstance{
			"team-a/pg-a": {Name: "pg-a", Namespace: "team-a", ServiceAccountEmailAddress: "sa-a@gcp.iam"},
			"team-b/pg-b": {Name: "pg-b", Namespace: "team-b", ServiceAccountEmailAddress: "sa-b@gcp.iam"},
		},
	}

	gcpMock := &mockGCP{
		existingObjects: map[string]bool{},
	}

	runner := NewRunnerWithInterfaces(k8sMock, gcpMock, "bucket")
	ctx := context.Background()

	err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gcpMock.exportCalls) != 2 {
		t.Errorf("expected 2 export calls, got %d", len(gcpMock.exportCalls))
	}
	if len(gcpMock.grantCalls) != 2 {
		t.Errorf("expected 2 grant calls, got %d", len(gcpMock.grantCalls))
	}
	if len(gcpMock.waitCalls) != 2 {
		t.Errorf("expected 2 wait calls, got %d", len(gcpMock.waitCalls))
	}
}
