// Package main is the entry point for the EdgeFabric Kubernetes operator.
package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	efv1alpha1 "github.com/jmcleod/edgefabric/operator/api/v1alpha1"
	"github.com/jmcleod/edgefabric/operator/internal/controller"
	"github.com/jmcleod/edgefabric/operator/pkg/efclient"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(efv1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var healthAddr string
	var efBaseURL string
	var efAPIKey string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&healthAddr, "health-probe-bind-address", ":8081", "The address the health probe endpoint binds to.")
	flag.StringVar(&efBaseURL, "edgefabric-url", os.Getenv("EDGEFABRIC_URL"), "EdgeFabric controller API base URL.")
	flag.StringVar(&efAPIKey, "edgefabric-api-key", os.Getenv("EDGEFABRIC_API_KEY"), "EdgeFabric API key for authentication.")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	setupLog := ctrl.Log.WithName("setup")

	if efBaseURL == "" {
		setupLog.Error(nil, "edgefabric-url is required (or set EDGEFABRIC_URL)")
		os.Exit(1)
	}

	efClient := efclient.New(efBaseURL, efAPIKey)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: healthAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Register all reconcilers.
	if err := (&controller.TenantReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		EFClient: efClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Tenant")
		os.Exit(1)
	}

	if err := (&controller.NodeReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		EFClient: efClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Node")
		os.Exit(1)
	}

	if err := (&controller.GatewayReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		EFClient: efClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Gateway")
		os.Exit(1)
	}

	if err := (&controller.DNSZoneReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		EFClient: efClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DNSZone")
		os.Exit(1)
	}

	if err := (&controller.CDNSiteReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		EFClient: efClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CDNSite")
		os.Exit(1)
	}

	if err := (&controller.RouteReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		EFClient: efClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Route")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
