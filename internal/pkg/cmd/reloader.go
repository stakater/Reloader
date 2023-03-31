package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/leadership"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	cmd.PersistentFlags().StringVar(&options.SearchMatchAnnotation, "search-match-annotation", "reloader.stakater.com/match", "annotation to mark secrets or configmaps to match the search")
	cmd.PersistentFlags().StringVar(&options.LogFormat, "log-format", "", "Log format to use (empty string for text, or JSON")
	cmd.PersistentFlags().StringSlice("resources-to-ignore", []string{}, "list of resources to ignore (valid options 'configMaps' or 'secrets')")
	cmd.PersistentFlags().StringSlice("namespaces-to-ignore", []string{}, "list of namespaces to ignore")
	cmd.PersistentFlags().StringSlice("namespace-selector", []string{}, "list of key:value labels to filter on for namespaces")
	cmd.PersistentFlags().StringSlice("resource-label-selector", []string{}, "list of key:value labels to filter on for configmaps and secrets")
	cmd.PersistentFlags().StringVar(&options.IsArgoRollouts, "is-Argo-Rollouts", "false", "Add support for argo rollouts")
	cmd.PersistentFlags().StringVar(&options.ReloadStrategy, constants.ReloadStrategyFlag, constants.EnvVarsReloadStrategy, "Specifies the desired reload strategy")
	cmd.PersistentFlags().StringVar(&options.ReloadOnCreate, "reload-on-create", "false", "Add support to watch create events")
	cmd.PersistentFlags().BoolVar(&options.EnableHA, "enable-ha", false, "Adds support for running multiple replicas via leadership election")
	cmd.PersistentFlags().BoolVar(&options.SyncAfterRestart, "sync-after-restart", false, "Sync add events after reloader restarts")

	return cmd
}

func validateFlags(*cobra.Command, []string) error {
	// Ensure the reload strategy is one of the following...
	var validReloadStrategy bool
	valid := []string{constants.EnvVarsReloadStrategy, constants.AnnotationsReloadStrategy}
	for _, s := range valid {
		if s == options.ReloadStrategy {
			validReloadStrategy = true
		}
	}

	if !validReloadStrategy {
		err := fmt.Sprintf("%s must be one of: %s", constants.ReloadStrategyFlag, strings.Join(valid, ", "))
		return errors.New(err)
	}

	// Validate that HA options are correct
	if options.EnableHA {
		if err := validateHAEnvs(); err != nil {
			return err
		}
	}

	return nil
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

func validateHAEnvs() error {
	podName, podNamespace := getHAEnvs()

	if podName == "" {
		return fmt.Errorf("%s not set, cannot run in HA mode without %s set", constants.PodNameEnv, constants.PodNameEnv)
	}
	if podNamespace == "" {
		return fmt.Errorf("%s not set, cannot run in HA mode without %s set", constants.PodNamespaceEnv, constants.PodNamespaceEnv)
	}
	return nil
}

func getHAEnvs() (string, string) {
	podName := os.Getenv(constants.PodNameEnv)
	podNamespace := os.Getenv(constants.PodNamespaceEnv)

	return podName, podNamespace
}

func startReloader(cmd *cobra.Command, args []string) {
	err := configureLogging(options.LogFormat)
	if err != nil {
		logrus.Warn(err)
	}

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

	namespaceLabelSelector, err := getNamespaceLabelSelector(cmd)
	if err != nil {
		logrus.Fatal(err)
	}

	resourceLabelSelector, err := getResourceLabelSelector(cmd)
	if err != nil {
		logrus.Fatal(err)
	}

	if len(namespaceLabelSelector) > 0 {
		logrus.Warnf("namespace-selector is set, will only detect changes in namespaces with these labels: %s.", namespaceLabelSelector)
	}

	if len(resourceLabelSelector) > 0 {
		logrus.Warnf("resource-label-selector is set, will only detect changes on resources with these labels: %s.", resourceLabelSelector)
	}

	collectors := metrics.SetupPrometheusEndpoint()

	var controllers []*controller.Controller
	for k := range kube.ResourceMap {
		if ignoredResourcesList.Contains(k) || (len(namespaceLabelSelector) == 0 && k == "namespaces") {
			continue
		}

		c, err := controller.NewController(clientset, k, currentNamespace, ignoredNamespacesList, namespaceLabelSelector, resourceLabelSelector, collectors)
		if err != nil {
			logrus.Fatalf("%s", err)
		}

		controllers = append(controllers, c)

		// If HA is enabled we only run the controller when
		if options.EnableHA {
			continue
		}
		// Now let's start the controller
		stop := make(chan struct{})
		defer close(stop)
		logrus.Infof("Starting Controller to watch resource type: %s", k)
		go c.Run(1, stop)
	}

	// Run leadership election
	if options.EnableHA {
		podName, podNamespace := getHAEnvs()
		lock := leadership.GetNewLock(clientset.CoordinationV1(), constants.LockName, podName, podNamespace)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go leadership.RunLeaderElection(lock, ctx, cancel, podName, controllers)
	}

	leadership.SetupLivenessEndpoint()
	logrus.Fatal(http.ListenAndServe(constants.DefaultHttpListenAddr, nil))
}

func getIgnoredNamespacesList(cmd *cobra.Command) (util.List, error) {
	return getStringSliceFromFlags(cmd, "namespaces-to-ignore")
}

func getNamespaceLabelSelector(cmd *cobra.Command) (string, error) {
	slice, err := getStringSliceFromFlags(cmd, "namespace-selector")
	if err != nil {
		logrus.Fatal(err)
	}

	for i, kv := range slice {
		// Legacy support for ":" as a delimiter and "*" for wildcard.
		if strings.Contains(kv, ":") {
			split := strings.Split(kv, ":")
			if split[1] == "*" {
				slice[i] = split[0]
			} else {
				slice[i] = split[0] + "=" + split[1]
			}
		}
		// Convert wildcard to valid apimachinery operator
		if strings.Contains(kv, "=") {
			split := strings.Split(kv, "=")
			if split[1] == "*" {
				slice[i] = split[0]
			}
		}
	}

	namespaceLabelSelector := strings.Join(slice[:], ",")
	_, err = v1.ParseToLabelSelector(namespaceLabelSelector)
	if err != nil {
		logrus.Fatal(err)
	}

	return namespaceLabelSelector, nil
}

func getResourceLabelSelector(cmd *cobra.Command) (string, error) {
	slice, err := getStringSliceFromFlags(cmd, "resource-label-selector")
	if err != nil {
		logrus.Fatal(err)
	}

	for i, kv := range slice {
		// Legacy support for ":" as a delimiter and "*" for wildcard.
		if strings.Contains(kv, ":") {
			split := strings.Split(kv, ":")
			if split[1] == "*" {
				slice[i] = split[0]
			} else {
				slice[i] = split[0] + "=" + split[1]
			}
		}
		// Convert wildcard to valid apimachinery operator
		if strings.Contains(kv, "=") {
			split := strings.Split(kv, "=")
			if split[1] == "*" {
				slice[i] = split[0]
			}
		}
	}

	resourceLabelSelector := strings.Join(slice[:], ",")
	_, err = v1.ParseToLabelSelector(resourceLabelSelector)
	if err != nil {
		logrus.Fatal(err)
	}

	return resourceLabelSelector, nil
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
