package cmd

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewReloaderCommand starts the reloader controller
func NewReloaderCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reloader",
		Short: "A watcher for your Kubernetes cluster",
		Run:   startReloader,
	}

	// options
	cmd.PersistentFlags().StringVar(&options.ConfigmapUpdateOnChangeAnnotation, "configmap-annotation", "configmap.reloader.stakater.com/reload", "annotation to detect changes in configmaps")
	cmd.PersistentFlags().StringVar(&options.SecretUpdateOnChangeAnnotation, "secret-annotation", "secret.reloader.stakater.com/reload", "annotation to detect changes in secrets")
	cmd.PersistentFlags().StringVar(&options.ReloaderAutoAnnotation, "auto-annotation", "reloader.stakater.com/auto", "annotation to detect changes in secrets")
	cmd.PersistentFlags().StringSlice("resources-to-watch", []string{"configMaps", "secrets"}, "list of resources to watch (valid options 'configMaps', 'secrets')")

	return cmd
}

func startReloader(cmd *cobra.Command, args []string) {
	var watchList util.List
	var err error

	logrus.Info("Starting Reloader")
	currentNamespace := os.Getenv("KUBERNETES_NAMESPACE")
	if len(currentNamespace) == 0 {
		currentNamespace = v1.NamespaceAll
		logrus.Warnf("KUBERNETES_NAMESPACE is unset, will detect changes in all namespaces.")
	}

	// create the clientset
	clientset, err := kube.GetClient()
	if err != nil {
		logrus.Fatal(err)
	}

	watchList, err = cmd.Flags().GetStringSlice("resources-to-watch")
	if err != nil {
		logrus.Fatal(err)
	}

	for _, v := range watchList {
		if v != "configMaps" && v != "secrets" {
			logrus.Fatalf("'resources-to-watch' only accepts 'configMaps' and 'secrets', not '%s'", v)
		}
	}

	for k := range kube.ResourceMap {
		if !watchList.Contains(k) {
			continue
		}

		c, err := controller.NewController(clientset, k, currentNamespace)
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
