package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/stakater/Reloader/internal/pkg/client"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	"github.com/golang/glog"
	oclient "github.com/openshift/origin/pkg/client"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/api"
	kubectlutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func NewReloaderCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:   "reloader",
		Short: "A watcher for your Kubernetes cluster",
		Run:   startReloader,
	}
	return cmds
}

const (
	healthPort = 10254
)

var (
	flags = pflag.NewFlagSet("", pflag.ExitOnError)

	resyncPeriod = flags.Duration("sync-period", 30*time.Second,
		`Relist and confirm services this often.`)

	healthzPort = flags.Int("healthz-port", healthPort, "port for healthz endpoint.")

	profiling = flags.Bool("profiling", true, `Enable profiling via web interface host:port/debug/pprof/`)
)

func startReloader(cmd *cobra.Command, args []string) {
	glog.Println("Starting Reloader")

	// create the clientset
	clientset, err := kube.GetClient()
	if err != nil {
		log.Fatal(err)
	}

	// get the Controller config file
	config := getControllerConfig()

	for k, v := range kube.ResourceMap {
		c, err := controller.NewController(clientset, *resyncPeriod, config, v)
		if err != nil {
			glog.Fatalf("%s", err)
		}

		go registerHandlers()
		go handleSigterm(c)

		// Now let's start the controller
		stop := make(chan struct{})
		defer close(stop)

		go c.Run(1, stop)
	}

	// Wait forever
	select {}
}

func registerHandlers() {
	mux := http.NewServeMux()

	if *profiling {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", *healthzPort),
		Handler: mux,
	}
	glog.Fatal(server.ListenAndServe())
}

func handleSigterm(c *controller.Controller) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-signalChan
	glog.Infof("Received %s, shutting down", sig)
	c.Stop()
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
		log.Panic(err)
	}
	return configuration
}
