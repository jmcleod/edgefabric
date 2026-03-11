package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	efv1alpha1 "github.com/jmcleod/edgefabric/operator/api/v1alpha1"
	"github.com/jmcleod/edgefabric/operator/pkg/efclient"
)

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = efv1alpha1.AddToScheme(s)
	return s
}

func TestTenantReconcile_Create(t *testing.T) {
	createCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants":
			createCalled = true
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(efclient.ResourceResponse{ID: "ef-tenant-123"})
		case r.Method == http.MethodPut:
			// Update after create is also valid.
			w.WriteHeader(http.StatusOK)
		default:
			t.Logf("request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	scheme := newTestScheme()
	tenant := &efv1alpha1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tenant",
			Namespace: "default",
		},
		Spec: efv1alpha1.TenantSpec{
			Name: "Test Tenant",
			Slug: "test-tenant",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(tenant).
		WithStatusSubresource(tenant).
		Build()

	reconciler := &TenantReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		EFClient: efclient.New(server.URL, "test-key"),
	}

	// First reconcile: adds finalizer, then creates in EdgeFabric API.
	result, err := reconciler.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-tenant", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	if result.Requeue {
		t.Error("unexpected requeue")
	}

	// Verify finalizer was added.
	var updated efv1alpha1.Tenant
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-tenant", Namespace: "default"}, &updated); err != nil {
		t.Fatalf("get tenant: %v", err)
	}

	hasFinalizer := false
	for _, f := range updated.Finalizers {
		if f == tenantFinalizer {
			hasFinalizer = true
		}
	}
	if !hasFinalizer {
		t.Error("expected finalizer to be added")
	}

	if !createCalled {
		t.Error("expected EdgeFabric API create to be called")
	}

	if updated.Status.ID != "ef-tenant-123" {
		t.Errorf("expected status ID ef-tenant-123, got %s", updated.Status.ID)
	}
	if updated.Status.Phase != "Ready" {
		t.Errorf("expected phase Ready, got %s", updated.Status.Phase)
	}
}

func TestTenantReconcile_Delete(t *testing.T) {
	deleteCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/ef-tenant-123":
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	scheme := newTestScheme()
	now := metav1.Now()
	tenant := &efv1alpha1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-tenant",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{tenantFinalizer},
		},
		Spec: efv1alpha1.TenantSpec{
			Name: "Test Tenant",
			Slug: "test-tenant",
		},
		Status: efv1alpha1.TenantStatus{
			ID:    "ef-tenant-123",
			Phase: "Ready",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(tenant).
		WithStatusSubresource(tenant).
		Build()

	reconciler := &TenantReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		EFClient: efclient.New(server.URL, "test-key"),
	}

	_, err := reconciler.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-tenant", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	if !deleteCalled {
		t.Error("expected EdgeFabric API delete to be called")
	}

	// After finalizer removal with DeletionTimestamp set, the fake client
	// deletes the object. Verify the object is gone (expected behavior).
	var updated efv1alpha1.Tenant
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-tenant", Namespace: "default"}, &updated)
	if err != nil && !errors.IsNotFound(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	// Either the object is gone (not found) or its finalizer was removed.
	if err == nil {
		for _, f := range updated.Finalizers {
			if f == tenantFinalizer {
				t.Error("expected finalizer to be removed")
			}
		}
	}
}

func TestTenantReconcile_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	scheme := newTestScheme()
	tenant := &efv1alpha1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-tenant",
			Namespace:  "default",
			Finalizers: []string{tenantFinalizer},
		},
		Spec: efv1alpha1.TenantSpec{
			Name: "Test Tenant",
			Slug: "test-tenant",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(tenant).
		WithStatusSubresource(tenant).
		Build()

	reconciler := &TenantReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		EFClient: efclient.New(server.URL, "test-key"),
	}

	_, err := reconciler.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "test-tenant", Namespace: "default"},
	})
	if err == nil {
		t.Fatal("expected error when API returns 500")
	}

	// Verify status shows failure.
	var updated efv1alpha1.Tenant
	if err := fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-tenant", Namespace: "default"}, &updated); err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	if updated.Status.Phase != "Failed" {
		t.Errorf("expected phase Failed, got %s", updated.Status.Phase)
	}
}
