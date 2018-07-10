package cmd

import (
	"github.com/spf13/cobra"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/pkg/kube"
	"github.com/sirupsen/logrus"
)

func NewReloaderCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:   "reloader",
		Short: "A watcher for your Kubernetes cluster",
		Run:   startReloader,
	}
	return cmds
}

func startReloader(cmd *cobra.Command, args []string) {
	logrus.Info("Starting Reloader")

	// create the clientset
	clientset, err := kube.GetClient()
	if err != nil {
		logrus.Fatal(err)
	}

	for k, v := range kube.ResourceMap {
		c, err := controller.NewController(clientset, k, v)
		if err != nil {
			logrus.Fatalf("%s", err)
		}

		// Now let's start the controller
		stop := make(chan struct{})
		defer close(stop)

		go c.Run(1, stop)
	}

	// Wait forever
	select {}
}
