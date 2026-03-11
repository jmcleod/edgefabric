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

const cdnsiteFinalizer = "edgefabric.io/cdnsite-cleanup"

// CDNSiteReconciler reconciles CDNSite CRs with the EdgeFabric API.
type CDNSiteReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	EFClient *efclient.Client
}

// +kubebuilder:rbac:groups=edgefabric.io,resources=cdnsites,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=edgefabric.io,resources=cdnsites/status,verbs=get;update;patch

func (r *CDNSiteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var site efv1alpha1.CDNSite
	if err := r.Get(ctx, req.NamespacedName, &site); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion.
	if !site.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&site, cdnsiteFinalizer) {
			if site.Status.ID != "" {
				if err := r.EFClient.DeleteCDNSite(ctx, site.Spec.TenantRef, site.Status.ID); err != nil {
					logger.Error(err, "failed to delete CDN site from EdgeFabric API")
					site.Status.Phase = "Failed"
					site.Status.Message = fmt.Sprintf("delete failed: %v", err)
					_ = r.Status().Update(ctx, &site)
					return ctrl.Result{}, err
				}
			}
			controllerutil.RemoveFinalizer(&site, cdnsiteFinalizer)
			if err := r.Update(ctx, &site); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer.
	if !controllerutil.ContainsFinalizer(&site, cdnsiteFinalizer) {
		controllerutil.AddFinalizer(&site, cdnsiteFinalizer)
		if err := r.Update(ctx, &site); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Convert spec origins to API origins.
	origins := make([]efclient.CDNOriginBody, len(site.Spec.Origins))
	for i, o := range site.Spec.Origins {
		origins[i] = efclient.CDNOriginBody{
			Address:             o.Address,
			Scheme:              o.Scheme,
			Weight:              o.Weight,
			HealthCheckPath:     o.HealthCheckPath,
			HealthCheckInterval: o.HealthCheckInterval,
		}
	}

	apiReq := efclient.CreateCDNSiteRequest{
		Name:               site.Spec.Name,
		TenantID:           site.Spec.TenantRef,
		Domains:            site.Spec.Domains,
		TLSMode:            site.Spec.TLSMode,
		CacheEnabled:       site.Spec.CacheEnabled,
		CacheTTL:           site.Spec.CacheTTL,
		CompressionEnabled: site.Spec.CompressionEnabled,
		RateLimitRPS:       site.Spec.RateLimitRPS,
		WAFEnabled:         site.Spec.WAFEnabled,
		WAFMode:            site.Spec.WAFMode,
		NodeGroupID:        site.Spec.NodeGroupRef,
		Origins:            origins,
	}

	// Create or update.
	if site.Status.ID == "" {
		// Create in EdgeFabric.
		resp, err := r.EFClient.CreateCDNSite(ctx, site.Spec.TenantRef, apiReq)
		if err != nil {
			logger.Error(err, "failed to create CDN site in EdgeFabric API")
			site.Status.Phase = "Failed"
			site.Status.Message = fmt.Sprintf("create failed: %v", err)
			_ = r.Status().Update(ctx, &site)
			return ctrl.Result{}, err
		}

		site.Status.ID = resp.ID
		site.Status.Phase = "Ready"
		site.Status.DomainCount = len(site.Spec.Domains)
		site.Status.Message = ""
		if err := r.Status().Update(ctx, &site); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("created CDN site", "id", resp.ID)
	} else {
		// Update in EdgeFabric.
		if err := r.EFClient.UpdateCDNSite(ctx, site.Spec.TenantRef, site.Status.ID, apiReq); err != nil {
			logger.Error(err, "failed to update CDN site in EdgeFabric API")
			site.Status.Phase = "Failed"
			site.Status.Message = fmt.Sprintf("update failed: %v", err)
			_ = r.Status().Update(ctx, &site)
			return ctrl.Result{}, err
		}

		site.Status.Phase = "Ready"
		site.Status.DomainCount = len(site.Spec.Domains)
		site.Status.Message = ""
		if err := r.Status().Update(ctx, &site); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CDNSiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&efv1alpha1.CDNSite{}).
		Complete(r)
}
