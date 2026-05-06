package k8s

import (
	"context"
	"fmt"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var (
	sqlDatabaseGVR = schema.GroupVersionResource{
		Group:    "sql.cnrm.cloud.google.com",
		Version:  "v1beta1",
		Resource: "sqldatabases",
	}
	sqlInstanceGVR = schema.GroupVersionResource{
		Group:    "sql.cnrm.cloud.google.com",
		Version:  "v1beta1",
		Resource: "sqlinstances",
	}
)

// SQLDatabase represents a CNRM SQLDatabase resource.
type SQLDatabase struct {
	Name        string
	Namespace   string
	InstanceRef string
	ResourceID  string
}

// SQLInstance represents a CNRM SQLInstance resource.
type SQLInstance struct {
	Name                       string
	Namespace                  string
	ServiceAccountEmailAddress string
}

// Client wraps a Kubernetes dynamic client for CNRM resources.
type Client struct {
	dynamic dynamic.Interface
}

// NewClient creates a new Kubernetes client using in-cluster config.
func NewClient() (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("getting in-cluster config: %w", err)
	}

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &Client{dynamic: dyn}, nil
}

// NewClientFromDynamic creates a Client from an existing dynamic.Interface (useful for testing).
func NewClientFromDynamic(dyn dynamic.Interface) *Client {
	return &Client{dynamic: dyn}
}

// GetNamespaceProjectID returns the CNRM project-id annotation for a namespace.
func (c *Client) GetNamespaceProjectID(ctx context.Context, namespace string) (string, error) {
	nsGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	ns, err := c.dynamic.Resource(nsGVR).Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting namespace %s: %w", namespace, err)
	}

	annotations := ns.GetAnnotations()
	projectID, ok := annotations["cnrm.cloud.google.com/project-id"]
	if !ok {
		return "", fmt.Errorf("namespace %s missing cnrm.cloud.google.com/project-id annotation", namespace)
	}

	return projectID, nil
}

// ListSQLDatabases returns all SQLDatabase resources across all namespaces.
func (c *Client) ListSQLDatabases(ctx context.Context) ([]SQLDatabase, error) {
	list, err := c.dynamic.Resource(sqlDatabaseGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing sqldatabases: %w", err)
	}

	var databases []SQLDatabase
	for _, item := range list.Items {
		db, err := parseSQLDatabase(item)
		if err != nil {
			slog.Warn("skipping malformed sqldatabase", "name", item.GetName(), "namespace", item.GetNamespace(), "error", err)
			continue
		}
		databases = append(databases, db)
	}

	return databases, nil
}

// GetSQLInstance returns a specific SQLInstance in a namespace.
// Returns (nil, nil) if the instance does not exist (NotFound).
// Returns (nil, error) for transient or permission errors.
func (c *Client) GetSQLInstance(ctx context.Context, namespace, name string) (*SQLInstance, error) {
	obj, err := c.dynamic.Resource(sqlInstanceGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting sqlinstance %s/%s: %w", namespace, name, err)
	}

	sa, found, err := unstructured.NestedString(obj.Object, "status", "serviceAccountEmailAddress")
	if err != nil {
		return nil, fmt.Errorf("reading status.serviceAccountEmailAddress from sqlinstance %s/%s: %w", namespace, name, err)
	}
	if !found || sa == "" {
		return nil, fmt.Errorf("sqlinstance %s/%s has no status.serviceAccountEmailAddress (instance may not be ready)", namespace, name)
	}

	return &SQLInstance{
		Name:                       obj.GetName(),
		Namespace:                  obj.GetNamespace(),
		ServiceAccountEmailAddress: sa,
	}, nil
}

func parseSQLDatabase(obj unstructured.Unstructured) (SQLDatabase, error) {
	instanceRef, _, err := unstructured.NestedString(obj.Object, "spec", "instanceRef", "name")
	if err != nil || instanceRef == "" {
		return SQLDatabase{}, fmt.Errorf("missing spec.instanceRef.name")
	}

	resourceID, _, _ := unstructured.NestedString(obj.Object, "spec", "resourceID")
	if resourceID == "" {
		resourceID = obj.GetName()
	}

	return SQLDatabase{
		Name:        obj.GetName(),
		Namespace:   obj.GetNamespace(),
		InstanceRef: instanceRef,
		ResourceID:  resourceID,
	}, nil
}
