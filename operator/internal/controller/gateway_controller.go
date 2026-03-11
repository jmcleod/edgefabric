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

const gatewayFinalizer = "edgefabric.io/gateway-cleanup"

// GatewayReconciler reconciles Gateway CRs with the EdgeFabric API.
type GatewayReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	EFClient *efclient.Client
}

// +kubebuilder:rbac:groups=edgefabric.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=edgefabric.io,resources=gateways/status,verbs=get;update;patch

func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var gateway efv1alpha1.Gateway
	if err := r.Get(ctx, req.NamespacedName, &gateway); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion.
	if !gateway.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&gateway, gatewayFinalizer) {
			if gateway.Status.ID != "" {
				if err := r.EFClient.DeleteGateway(ctx, gateway.Status.ID); err != nil {
					logger.Error(err, "failed to delete gateway from EdgeFabric API")
					gateway.Status.Phase = "Failed"
					gateway.Status.Message = fmt.Sprintf("delete failed: %v", err)
					_ = r.Status().Update(ctx, &gateway)
					return ctrl.Result{}, err
				}
			}
			controllerutil.RemoveFinalizer(&gateway, gatewayFinalizer)
			if err := r.Update(ctx, &gateway); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer.
	if !controllerutil.ContainsFinalizer(&gateway, gatewayFinalizer) {
		controllerutil.AddFinalizer(&gateway, gatewayFinalizer)
		if err := r.Update(ctx, &gateway); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Create or update.
	if gateway.Status.ID == "" {
		// Create in EdgeFabric.
		resp, err := r.EFClient.CreateGateway(ctx, efclient.CreateGatewayRequest{
			Name:     gateway.Spec.Name,
			PublicIP:  gateway.Spec.PublicIP,
			TenantID: gateway.Spec.TenantRef,
		})
		if err != nil {
			logger.Error(err, "failed to create gateway in EdgeFabric API")
			gateway.Status.Phase = "Failed"
			gateway.Status.Message = fmt.Sprintf("create failed: %v", err)
			_ = r.Status().Update(ctx, &gateway)
			return ctrl.Result{}, err
		}

		gateway.Status.ID = resp.ID
		gateway.Status.Phase = "Ready"
		gateway.Status.Message = ""
		if err := r.Status().Update(ctx, &gateway); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("created gateway", "id", resp.ID)
	} else {
		// Update in EdgeFabric.
		if err := r.EFClient.UpdateGateway(ctx, gateway.Status.ID, efclient.CreateGatewayRequest{
			Name:     gateway.Spec.Name,
			PublicIP:  gateway.Spec.PublicIP,
			TenantID: gateway.Spec.TenantRef,
		}); err != nil {
			logger.Error(err, "failed to update gateway in EdgeFabric API")
			gateway.Status.Phase = "Failed"
			gateway.Status.Message = fmt.Sprintf("update failed: %v", err)
			_ = r.Status().Update(ctx, &gateway)
			return ctrl.Result{}, err
		}

		gateway.Status.Phase = "Ready"
		gateway.Status.Message = ""
		if err := r.Status().Update(ctx, &gateway); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&efv1alpha1.Gateway{}).
		Complete(r)
}
