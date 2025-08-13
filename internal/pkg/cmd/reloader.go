package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/leadership"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/common"
	"github.com/stakater/Reloader/pkg/kube"
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
	util.ConfigureReloaderFlags(cmd)

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

func configureLogging(logFormat, logLevel string) error {
	switch logFormat {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	default:
		// just let the library use default on empty string.
		if logFormat != "" {
			return fmt.Errorf("unsupported logging formatter: %q", logFormat)
		}
	}
	// set log level
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return err
	}
	logrus.SetLevel(level)
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
	common.GetCommandLineOptions()
	err := configureLogging(options.LogFormat, options.LogLevel)
	if err != nil {
		logrus.Warn(err)
	}

	logrus.Info("Starting Reloader")
	isGlobal := false
	currentNamespace := os.Getenv("KUBERNETES_NAMESPACE")
	if len(currentNamespace) == 0 {
		currentNamespace = v1.NamespaceAll
		isGlobal = true
		logrus.Warnf("KUBERNETES_NAMESPACE is unset, will detect changes in all namespaces.")
	}

	// create the clientset
	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		logrus.Fatal(err)
	}

	ignoredResourcesList, err := util.GetIgnoredResourcesList()
	if err != nil {
		logrus.Fatal(err)
	}

	ignoredNamespacesList := options.NamespacesToIgnore
	namespaceLabelSelector := ""

	if isGlobal {
		namespaceLabelSelector, err = common.GetNamespaceLabelSelector(options.NamespaceSelectors)
		if err != nil {
			logrus.Fatal(err)
		}
	}

	resourceLabelSelector, err := common.GetResourceLabelSelector(options.ResourceSelectors)
	if err != nil {
		logrus.Fatal(err)
	}

	if len(namespaceLabelSelector) > 0 {
		logrus.Warnf("namespace-selector is set, will only detect changes in namespaces with these labels: %s.", namespaceLabelSelector)
	}

	if len(resourceLabelSelector) > 0 {
		logrus.Warnf("resource-label-selector is set, will only detect changes on resources with these labels: %s.", resourceLabelSelector)
	}

	if options.WebhookUrl != "" {
		logrus.Warnf("webhook-url is set, will only send webhook, no resources will be reloaded")
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

	common.PublishMetaInfoConfigmap(clientset)

	if options.EnablePProf {
		go startPProfServer()
	}

	leadership.SetupLivenessEndpoint()
	logrus.Fatal(http.ListenAndServe(constants.DefaultHttpListenAddr, nil))
}

func startPProfServer() {
	logrus.Infof("Starting pprof server on %s", options.PProfAddr)
	if err := http.ListenAndServe(options.PProfAddr, nil); err != nil {
		logrus.Errorf("Failed to start pprof server: %v", err)
	}
}
