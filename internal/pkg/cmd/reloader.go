package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"os"
	"strings"
	"time"
)

var (
	electionClient *clientset.Clientset
)

// NewReloaderCommand starts the reloader controller
func NewReloaderCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "reloader",
		Short:   "A watcher for your Kubernetes cluster",
		PreRunE: validateFlags,
		Run:     startReloader,
	}

	// options
	cmd.PersistentFlags().StringVar(&options.ConfigmapUpdateOnChangeAnnotation, "configmap-annotation", "configmap.reloader.stakater.com/reload", "annotation to detect changes in configmaps, specified by name")
	cmd.PersistentFlags().StringVar(&options.SecretUpdateOnChangeAnnotation, "secret-annotation", "secret.reloader.stakater.com/reload", "annotation to detect changes in secrets, specified by name")
	cmd.PersistentFlags().StringVar(&options.ReloaderAutoAnnotation, "auto-annotation", "reloader.stakater.com/auto", "annotation to detect changes in secrets")
	cmd.PersistentFlags().StringVar(&options.AutoSearchAnnotation, "auto-search-annotation", "reloader.stakater.com/search", "annotation to detect changes in configmaps or secrets tagged with special match annotation")
	cmd.PersistentFlags().StringVar(&options.SearchMatchAnnotation, "search-match-annotation", "reloader.stakater.com/match", "annotation to mark secrets or configmapts to match the search")
	cmd.PersistentFlags().StringVar(&options.LogFormat, "log-format", "", "Log format to use (empty string for text, or JSON")
	cmd.PersistentFlags().StringSlice("resources-to-ignore", []string{}, "list of resources to ignore (valid options 'configMaps' or 'secrets')")
	cmd.PersistentFlags().StringSlice("namespaces-to-ignore", []string{}, "list of namespaces to ignore")
	cmd.PersistentFlags().StringVar(&options.IsArgoRollouts, "is-Argo-Rollouts", "false", "Add support for argo rollouts")
	cmd.PersistentFlags().StringVar(&options.ReloadStrategy, constants.ReloadStrategyFlag, constants.EnvVarsReloadStrategy, "Specifies the desired reload strategy")
	cmd.PersistentFlags().StringVar(&options.ReloadOnCreate, "reload-on-create", "false", "Add support to watch create events")
	cmd.PersistentFlags().BoolVar(&options.HAEnabled, "ha", false, "Whether to run in HA")
	cmd.PersistentFlags().StringVar(&options.LeaseLockName, "lease-name", "stakater-reloader-lease", "Name of the lease object to use for leader election")
	cmd.PersistentFlags().StringVar(&options.LeaseLockNameSpace, "lease-namespace", "default", "Namespace of the Lease resource")
	return cmd
}

func validateFlags(*cobra.Command, []string) error {
	// Ensure the reload strategy is one of the following...
	valid := []string{constants.EnvVarsReloadStrategy, constants.AnnotationsReloadStrategy}
	for _, s := range valid {
		if s == options.ReloadStrategy {
			return nil
		}
	}

	err := fmt.Sprintf("%s must be one of: %s", constants.ReloadStrategyFlag, strings.Join(valid, ", "))
	return errors.New(err)
}

func configureLogging(logFormat string) error {
	switch logFormat {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	default:
		// just let the library use default on empty string.
		if logFormat != "" {
			return fmt.Errorf("unsupported logging formatter: %q", logFormat)
		}
	}
	return nil
}

func getNewLock(lockname, podname, namespace string) *resourcelock.LeaseLock {
	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      lockname,
			Namespace: namespace,
		},
		Client: electionClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: podname,
		},
	}
}

func startLeaderElection(cmd *cobra.Command) {
	config, err := rest.InClusterConfig()
	electionClient = clientset.NewForConfigOrDie(config)
	var podName string = os.Getenv("POD_NAME")
	if err != nil {
		logrus.Fatal("failed to get kubeconfig")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lock := getNewLock(options.LeaseLockName, podName, options.LeaseLockNameSpace)
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(c context.Context) {
				start(cmd)
			},
			OnStoppedLeading: func() {
				for {
					logrus.Info("no longer the leader, staying inactive.")
				}
			},
			OnNewLeader: func(current_id string) {
				if current_id == podName {
					logrus.Info("still the leader!")
					return
				}
				logrus.Info("new leader is %s", current_id)
				metrics.SetupPrometheusEndpoint()
			},
		},
	})
}

func start(cmd *cobra.Command) {
	logrus.Info("Starting Reloader")
	currentNamespace := os.Getenv("KUBERNETES_NAMESPACE")
	if len(currentNamespace) == 0 {
		currentNamespace = v1.NamespaceAll
		logrus.Warnf("KUBERNETES_NAMESPACE is unset, will detect changes in all namespaces.")
	}

	// create the clientset
	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		logrus.Fatal(err)
	}

	ignoredResourcesList, err := getIgnoredResourcesList(cmd)
	if err != nil {
		logrus.Fatal(err)
	}

	ignoredNamespacesList, err := getIgnoredNamespacesList(cmd)
	if err != nil {
		logrus.Fatal(err)
	}

	collectors := metrics.SetupPrometheusEndpoint()

	for k := range kube.ResourceMap {
		if ignoredResourcesList.Contains(k) {
			continue
		}

		c, err := controller.NewController(clientset, k, currentNamespace, ignoredNamespacesList, collectors)
		if err != nil {
			logrus.Fatalf("%s", err)
		}

		// Now let's start the controller
		stop := make(chan struct{})
		defer close(stop)
		logrus.Infof("Starting Controller to watch resource type: %s", k)
		go c.Run(1, stop)
	}

	// Wait forever
	select {}
}

func startReloader(cmd *cobra.Command, args []string) {
	err := configureLogging(options.LogFormat)
	if err != nil {
		logrus.Warn(err)
	}

	if options.HAEnabled {
		startLeaderElection(cmd)
	} else {
		start(cmd)
	}

}

func getIgnoredNamespacesList(cmd *cobra.Command) (util.List, error) {
	return getStringSliceFromFlags(cmd, "namespaces-to-ignore")
}

func getStringSliceFromFlags(cmd *cobra.Command, flag string) ([]string, error) {
	slice, err := cmd.Flags().GetStringSlice(flag)
	if err != nil {
		return nil, err
	}

	return slice, nil
}

func getIgnoredResourcesList(cmd *cobra.Command) (util.List, error) {

	ignoredResourcesList, err := getStringSliceFromFlags(cmd, "resources-to-ignore")
	if err != nil {
		return nil, err
	}

	for _, v := range ignoredResourcesList {
		if v != "configMaps" && v != "secrets" {
			return nil, fmt.Errorf("'resources-to-ignore' only accepts 'configMaps' or 'secrets', not '%s'", v)
		}
	}

	if len(ignoredResourcesList) > 1 {
		return nil, errors.New("'resources-to-ignore' only accepts 'configMaps' or 'secrets', not both")
	}

	return ignoredResourcesList, nil
}
