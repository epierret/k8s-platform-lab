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
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	platformv1alpha1 "github.com/epierret/k8s-platform-lab/api/v1alpha1"
	"github.com/epierret/k8s-platform-lab/internal/controller"
)

// Injectés au build par -ldflags (leçon 02).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var probeAddr, metricsAddr string
	var leaderElect bool
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Adresse des sondes /healthz et /readyz.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "Adresse des métriques (0 = désactivées).")
	flag.BoolVar(&leaderElect, "leader-elect", false, "Un seul opérateur actif à la fois (à activer dans le cluster).")
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log.WithName("setup")

	// Le namespace de l'opérateur : là où vivent le registre et le verrou de leader election.
	ns := os.Getenv("POD_NAMESPACE")
	if ns == "" {
		log.Error(nil, "POD_NAMESPACE must be set")
		os.Exit(1)
	}

	// Le "dictionnaire" des types que le client connaît : ceux de Kubernetes + WebApp.
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(platformv1alpha1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          leaderElect,
		LeaderElectionID:        "webapp-operator.platform.enrique-pierret.com",
		LeaderElectionNamespace: ns,
	})
	if err != nil {
		log.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err := (&controller.WebAppReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		RegistryNamespace: ns,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to set up controller")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	log.Info("starting manager", "version", version, "commit", commit, "date", date)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "manager exited with error")
		os.Exit(1)
	}
}
