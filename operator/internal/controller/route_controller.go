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

const routeFinalizer = "edgefabric.io/route-cleanup"

// RouteReconciler reconciles Route CRs with the EdgeFabric API.
type RouteReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	EFClient *efclient.Client
}

// +kubebuilder:rbac:groups=edgefabric.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=edgefabric.io,resources=routes/status,verbs=get;update;patch

func (r *RouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var route efv1alpha1.Route
	if err := r.Get(ctx, req.NamespacedName, &route); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion.
	if !route.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&route, routeFinalizer) {
			if route.Status.ID != "" {
				if err := r.EFClient.DeleteRoute(ctx, route.Spec.TenantRef, route.Status.ID); err != nil {
					logger.Error(err, "failed to delete route from EdgeFabric API")
					route.Status.Phase = "Failed"
					route.Status.Message = fmt.Sprintf("delete failed: %v", err)
					_ = r.Status().Update(ctx, &route)
					return ctrl.Result{}, err
				}
			}
			controllerutil.RemoveFinalizer(&route, routeFinalizer)
			if err := r.Update(ctx, &route); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer.
	if !controllerutil.ContainsFinalizer(&route, routeFinalizer) {
		controllerutil.AddFinalizer(&route, routeFinalizer)
		if err := r.Update(ctx, &route); err != nil {
			return ctrl.Result{}, err
		}
	}

	apiReq := efclient.CreateRouteRequest{
		Name:            route.Spec.Name,
		TenantID:        route.Spec.TenantRef,
		Protocol:        route.Spec.Protocol,
		EntryIP:         route.Spec.EntryIP,
		EntryPort:       route.Spec.EntryPort,
		DestinationIP:   route.Spec.DestinationIP,
		DestinationPort: route.Spec.DestinationPort,
		GatewayID:       route.Spec.GatewayRef,
	}

	// Create or update.
	if route.Status.ID == "" {
		// Create in EdgeFabric.
		resp, err := r.EFClient.CreateRoute(ctx, route.Spec.TenantRef, apiReq)
		if err != nil {
			logger.Error(err, "failed to create route in EdgeFabric API")
			route.Status.Phase = "Failed"
			route.Status.Message = fmt.Sprintf("create failed: %v", err)
			_ = r.Status().Update(ctx, &route)
			return ctrl.Result{}, err
		}

		route.Status.ID = resp.ID
		route.Status.Phase = "Ready"
		route.Status.Message = ""
		if err := r.Status().Update(ctx, &route); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("created route", "id", resp.ID)
	} else {
		// Update in EdgeFabric.
		if err := r.EFClient.UpdateRoute(ctx, route.Spec.TenantRef, route.Status.ID, apiReq); err != nil {
			logger.Error(err, "failed to update route in EdgeFabric API")
			route.Status.Phase = "Failed"
			route.Status.Message = fmt.Sprintf("update failed: %v", err)
			_ = r.Status().Update(ctx, &route)
			return ctrl.Result{}, err
		}

		route.Status.Phase = "Ready"
		route.Status.Message = ""
		if err := r.Status().Update(ctx, &route); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&efv1alpha1.Route{}).
		Complete(r)
}
