package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/pkg/kube"
	"github.com/stakater/Reloader/internal/pkg/config"
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

	// get the Controller config file
	config := getControllerConfig()

	for _, v := range kube.ResourceMap {
		c, err := controller.NewController(clientset, config.Controllers[0], v)
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

// get the yaml configuration for the controller
func getControllerConfig() config.Config {
	configFilePath := os.Getenv("CONFIG_FILE_PATH")
	if len(configFilePath) == 0 {
		//Default config file is placed in configs/ folder
		configFilePath = "configs/config.yaml"
	}
	configuration, err := config.ReadConfig(configFilePath)
	if err != nil {
		logrus.Panic(err)
	}
	return configuration
}
