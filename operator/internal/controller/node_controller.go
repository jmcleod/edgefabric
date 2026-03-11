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

const nodeFinalizer = "edgefabric.io/node-cleanup"

// NodeReconciler reconciles Node CRs with the EdgeFabric API.
type NodeReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	EFClient *efclient.Client
}

// +kubebuilder:rbac:groups=edgefabric.io,resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=edgefabric.io,resources=nodes/status,verbs=get;update;patch

func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var node efv1alpha1.Node
	if err := r.Get(ctx, req.NamespacedName, &node); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion.
	if !node.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&node, nodeFinalizer) {
			if node.Status.ID != "" {
				if err := r.EFClient.DeleteNode(ctx, node.Status.ID); err != nil {
					logger.Error(err, "failed to delete node from EdgeFabric API")
					node.Status.Phase = "Failed"
					node.Status.Message = fmt.Sprintf("delete failed: %v", err)
					_ = r.Status().Update(ctx, &node)
					return ctrl.Result{}, err
				}
			}
			controllerutil.RemoveFinalizer(&node, nodeFinalizer)
			if err := r.Update(ctx, &node); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer.
	if !controllerutil.ContainsFinalizer(&node, nodeFinalizer) {
		controllerutil.AddFinalizer(&node, nodeFinalizer)
		if err := r.Update(ctx, &node); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Create or update.
	if node.Status.ID == "" {
		// Create in EdgeFabric.
		resp, err := r.EFClient.CreateNode(ctx, efclient.CreateNodeRequest{
			Name:     node.Spec.Name,
			Hostname: node.Spec.Hostname,
			Region:   node.Spec.Region,
			PublicIP:  node.Spec.PublicIP,
			TenantID: node.Spec.TenantRef,
		})
		if err != nil {
			logger.Error(err, "failed to create node in EdgeFabric API")
			node.Status.Phase = "Failed"
			node.Status.Message = fmt.Sprintf("create failed: %v", err)
			_ = r.Status().Update(ctx, &node)
			return ctrl.Result{}, err
		}

		node.Status.ID = resp.ID
		node.Status.Phase = "Ready"
		node.Status.Message = ""
		if err := r.Status().Update(ctx, &node); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("created node", "id", resp.ID)
	} else {
		// Update in EdgeFabric.
		if err := r.EFClient.UpdateNode(ctx, node.Status.ID, efclient.CreateNodeRequest{
			Name:     node.Spec.Name,
			Hostname: node.Spec.Hostname,
			Region:   node.Spec.Region,
			PublicIP:  node.Spec.PublicIP,
			TenantID: node.Spec.TenantRef,
		}); err != nil {
			logger.Error(err, "failed to update node in EdgeFabric API")
			node.Status.Phase = "Failed"
			node.Status.Message = fmt.Sprintf("update failed: %v", err)
			_ = r.Status().Update(ctx, &node)
			return ctrl.Result{}, err
		}

		node.Status.Phase = "Ready"
		node.Status.Message = ""
		if err := r.Status().Update(ctx, &node); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&efv1alpha1.Node{}).
		Complete(r)
}
