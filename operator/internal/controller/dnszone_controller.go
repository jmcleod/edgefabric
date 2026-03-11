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

const dnszoneFinalizer = "edgefabric.io/dnszone-cleanup"

// DNSZoneReconciler reconciles DNSZone CRs with the EdgeFabric API.
type DNSZoneReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	EFClient *efclient.Client
}

// +kubebuilder:rbac:groups=edgefabric.io,resources=dnszones,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=edgefabric.io,resources=dnszones/status,verbs=get;update;patch

func (r *DNSZoneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var zone efv1alpha1.DNSZone
	if err := r.Get(ctx, req.NamespacedName, &zone); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion.
	if !zone.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&zone, dnszoneFinalizer) {
			if zone.Status.ID != "" {
				if err := r.EFClient.DeleteDNSZone(ctx, zone.Spec.TenantRef, zone.Status.ID); err != nil {
					logger.Error(err, "failed to delete DNS zone from EdgeFabric API")
					zone.Status.Phase = "Failed"
					zone.Status.Message = fmt.Sprintf("delete failed: %v", err)
					_ = r.Status().Update(ctx, &zone)
					return ctrl.Result{}, err
				}
			}
			controllerutil.RemoveFinalizer(&zone, dnszoneFinalizer)
			if err := r.Update(ctx, &zone); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer.
	if !controllerutil.ContainsFinalizer(&zone, dnszoneFinalizer) {
		controllerutil.AddFinalizer(&zone, dnszoneFinalizer)
		if err := r.Update(ctx, &zone); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Convert spec records to API records.
	records := make([]efclient.DNSRecordBody, len(zone.Spec.Records))
	for i, rec := range zone.Spec.Records {
		records[i] = efclient.DNSRecordBody{
			Name:     rec.Name,
			Type:     rec.Type,
			Value:    rec.Value,
			TTL:      rec.TTL,
			Priority: rec.Priority,
		}
	}

	// Create or update.
	if zone.Status.ID == "" {
		// Create in EdgeFabric.
		resp, err := r.EFClient.CreateDNSZone(ctx, zone.Spec.TenantRef, efclient.CreateDNSZoneRequest{
			Name:     zone.Spec.Name,
			TenantID: zone.Spec.TenantRef,
			Records:  records,
		})
		if err != nil {
			logger.Error(err, "failed to create DNS zone in EdgeFabric API")
			zone.Status.Phase = "Failed"
			zone.Status.Message = fmt.Sprintf("create failed: %v", err)
			_ = r.Status().Update(ctx, &zone)
			return ctrl.Result{}, err
		}

		zone.Status.ID = resp.ID
		zone.Status.Phase = "Ready"
		zone.Status.RecordCount = len(zone.Spec.Records)
		zone.Status.Message = ""
		if err := r.Status().Update(ctx, &zone); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("created DNS zone", "id", resp.ID)
	} else {
		// Update in EdgeFabric.
		if err := r.EFClient.UpdateDNSZone(ctx, zone.Spec.TenantRef, zone.Status.ID, efclient.CreateDNSZoneRequest{
			Name:     zone.Spec.Name,
			TenantID: zone.Spec.TenantRef,
			Records:  records,
		}); err != nil {
			logger.Error(err, "failed to update DNS zone in EdgeFabric API")
			zone.Status.Phase = "Failed"
			zone.Status.Message = fmt.Sprintf("update failed: %v", err)
			_ = r.Status().Update(ctx, &zone)
			return ctrl.Result{}, err
		}

		zone.Status.Phase = "Ready"
		zone.Status.RecordCount = len(zone.Spec.Records)
		zone.Status.Message = ""
		if err := r.Status().Update(ctx, &zone); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DNSZoneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&efv1alpha1.DNSZone{}).
		Complete(r)
}
