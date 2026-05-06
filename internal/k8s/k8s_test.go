package k8s

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestParseSQLDatabase(t *testing.T) {
	tests := []struct {
		name    string
		obj     unstructured.Unstructured
		want    SQLDatabase
		wantErr bool
	}{
		{
			name: "valid database with resourceID",
			obj: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "sql.cnrm.cloud.google.com/v1beta1",
					"kind":       "SQLDatabase",
					"metadata": map[string]interface{}{
						"name":      "my-db",
						"namespace": "my-team",
					},
					"spec": map[string]interface{}{
						"instanceRef": map[string]interface{}{
							"name": "my-instance",
						},
						"resourceID": "actual_db_name",
					},
				},
			},
			want: SQLDatabase{
				Name:        "my-db",
				Namespace:   "my-team",
				InstanceRef: "my-instance",
				ResourceID:  "actual_db_name",
			},
		},
		{
			name: "valid database without resourceID falls back to name",
			obj: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "sql.cnrm.cloud.google.com/v1beta1",
					"kind":       "SQLDatabase",
					"metadata": map[string]interface{}{
						"name":      "my-db",
						"namespace": "my-team",
					},
					"spec": map[string]interface{}{
						"instanceRef": map[string]interface{}{
							"name": "my-instance",
						},
					},
				},
			},
			want: SQLDatabase{
				Name:        "my-db",
				Namespace:   "my-team",
				InstanceRef: "my-instance",
				ResourceID:  "my-db",
			},
		},
		{
			name: "missing instanceRef",
			obj: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "sql.cnrm.cloud.google.com/v1beta1",
					"kind":       "SQLDatabase",
					"metadata": map[string]interface{}{
						"name":      "bad-db",
						"namespace": "my-team",
					},
					"spec": map[string]interface{}{},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSQLDatabase(tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSQLDatabase() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseSQLDatabase() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetNamespaceProjectID(t *testing.T) {
	ns := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": "my-team",
				"annotations": map[string]interface{}{
					"cnrm.cloud.google.com/project-id": "my-gcp-project",
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "namespaces"}: "NamespaceList",
		},
		ns,
	)

	client := NewClientFromDynamic(dynClient)
	ctx := context.Background()

	projectID, err := client.GetNamespaceProjectID(ctx, "my-team")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projectID != "my-gcp-project" {
		t.Errorf("got projectID %q, want %q", projectID, "my-gcp-project")
	}
}

func TestGetNamespaceProjectID_MissingAnnotation(t *testing.T) {
	ns := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": "no-annotation",
			},
		},
	}

	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "namespaces"}: "NamespaceList",
		},
		ns,
	)

	client := NewClientFromDynamic(dynClient)
	ctx := context.Background()

	_, err := client.GetNamespaceProjectID(ctx, "no-annotation")
	if err == nil {
		t.Fatal("expected error for missing annotation, got nil")
	}
}

func TestListSQLDatabases(t *testing.T) {
	db1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "sql.cnrm.cloud.google.com/v1beta1",
			"kind":       "SQLDatabase",
			"metadata": map[string]interface{}{
				"name":      "app-db",
				"namespace": "team-a",
			},
			"spec": map[string]interface{}{
				"instanceRef": map[string]interface{}{
					"name": "pg-instance-1",
				},
				"resourceID": "appdb",
			},
		},
	}

	db2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "sql.cnrm.cloud.google.com/v1beta1",
			"kind":       "SQLDatabase",
			"metadata": map[string]interface{}{
				"name":      "other-db",
				"namespace": "team-b",
			},
			"spec": map[string]interface{}{
				"instanceRef": map[string]interface{}{
					"name": "pg-instance-2",
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			sqlDatabaseGVR: "SQLDatabaseList",
		},
		db1, db2,
	)

	client := NewClientFromDynamic(dynClient)
	ctx := context.Background()

	databases, err := client.ListSQLDatabases(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(databases) != 2 {
		t.Fatalf("got %d databases, want 2", len(databases))
	}

	// Find team-a db
	var found bool
	for _, db := range databases {
		if db.Name == "app-db" {
			found = true
			if db.InstanceRef != "pg-instance-1" {
				t.Errorf("got InstanceRef %q, want %q", db.InstanceRef, "pg-instance-1")
			}
			if db.ResourceID != "appdb" {
				t.Errorf("got ResourceID %q, want %q", db.ResourceID, "appdb")
			}
		}
	}
	if !found {
		t.Error("app-db not found in results")
	}
}

func TestGetSQLInstance_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			sqlInstanceGVR: "SQLInstanceList",
		},
	)

	client := NewClientFromDynamic(dynClient)
	ctx := context.Background()

	got, err := client.GetSQLInstance(ctx, "my-team", "nonexistent")
	if err != nil {
		t.Fatalf("expected nil error for NotFound, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil instance for NotFound, got: %+v", got)
	}
}

func TestGetSQLInstance(t *testing.T) {
	instance := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "sql.cnrm.cloud.google.com/v1beta1",
			"kind":       "SQLInstance",
			"metadata": map[string]interface{}{
				"name":      "my-instance",
				"namespace": "my-team",
			},
			"status": map[string]interface{}{
				"serviceAccountEmailAddress": "sa@gcp.iam.gserviceaccount.com",
			},
		},
	}

	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			sqlInstanceGVR: "SQLInstanceList",
		},
		instance,
	)

	client := NewClientFromDynamic(dynClient)
	ctx := context.Background()

	got, err := client.GetSQLInstance(ctx, "my-team", "my-instance")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "my-instance" {
		t.Errorf("got Name %q, want %q", got.Name, "my-instance")
	}
	if got.ServiceAccountEmailAddress != "sa@gcp.iam.gserviceaccount.com" {
		t.Errorf("got SA %q, want %q", got.ServiceAccountEmailAddress, "sa@gcp.iam.gserviceaccount.com")
	}
}
