package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	efv1alpha1 "github.com/jmcleod/edgefabric/operator/api/v1alpha1"
	"github.com/jmcleod/edgefabric/operator/pkg/efclient"
)

const tenantFinalizer = "edgefabric.io/tenant-cleanup"

// TenantReconciler reconciles Tenant CRs with the EdgeFabric API.
type TenantReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	EFClient *efclient.Client
}

// +kubebuilder:rbac:groups=edgefabric.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=edgefabric.io,resources=tenants/status,verbs=get;update;patch

func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var tenant efv1alpha1.Tenant
	if err := r.Get(ctx, req.NamespacedName, &tenant); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion.
	if !tenant.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&tenant, tenantFinalizer) {
			if tenant.Status.ID != "" {
				if err := r.EFClient.DeleteTenant(ctx, tenant.Status.ID); err != nil {
					logger.Error(err, "failed to delete tenant from EdgeFabric API")
					tenant.Status.Phase = "Failed"
					tenant.Status.Message = fmt.Sprintf("delete failed: %v", err)
					_ = r.Status().Update(ctx, &tenant)
					return ctrl.Result{}, err
				}
			}
			controllerutil.RemoveFinalizer(&tenant, tenantFinalizer)
			if err := r.Update(ctx, &tenant); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer.
	if !controllerutil.ContainsFinalizer(&tenant, tenantFinalizer) {
		controllerutil.AddFinalizer(&tenant, tenantFinalizer)
		if err := r.Update(ctx, &tenant); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Create or update.
	if tenant.Status.ID == "" {
		// Create in EdgeFabric.
		resp, err := r.EFClient.CreateTenant(ctx, efclient.CreateTenantRequest{
			Name: tenant.Spec.Name,
			Slug: tenant.Spec.Slug,
		})
		if err != nil {
			logger.Error(err, "failed to create tenant in EdgeFabric API")
			tenant.Status.Phase = "Failed"
			tenant.Status.Message = fmt.Sprintf("create failed: %v", err)
			_ = r.Status().Update(ctx, &tenant)
			return ctrl.Result{}, err
		}

		tenant.Status.ID = resp.ID
		tenant.Status.Phase = "Ready"
		tenant.Status.Message = ""
		if err := r.Status().Update(ctx, &tenant); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("created tenant", "id", resp.ID)
	} else {
		// Update in EdgeFabric.
		if err := r.EFClient.UpdateTenant(ctx, tenant.Status.ID, efclient.CreateTenantRequest{
			Name: tenant.Spec.Name,
			Slug: tenant.Spec.Slug,
		}); err != nil {
			logger.Error(err, "failed to update tenant in EdgeFabric API")
			tenant.Status.Phase = "Failed"
			tenant.Status.Message = fmt.Sprintf("update failed: %v", err)
			_ = r.Status().Update(ctx, &tenant)
			return ctrl.Result{}, err
		}

		tenant.Status.Phase = "Ready"
		tenant.Status.Message = ""
		if err := r.Status().Update(ctx, &tenant); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&efv1alpha1.Tenant{}).
		Complete(r)
}
