package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/stakater/Reloader/test/loadtest/internal/cluster"
	"github.com/stakater/Reloader/test/loadtest/internal/prometheus"
	"github.com/stakater/Reloader/test/loadtest/internal/reloader"
	"github.com/stakater/Reloader/test/loadtest/internal/scenarios"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// RunConfig holds CLI configuration for the run command.
type RunConfig struct {
	OldImage     string
	NewImage     string
	Scenario     string
	Duration     int
	SkipCluster  bool
	ClusterName  string
	ResultsDir   string
	ManifestsDir string
	Parallelism  int
}

// workerContext holds all resources for a single worker (cluster + prometheus).
type workerContext struct {
	id          int
	clusterMgr  *cluster.Manager
	promMgr     *prometheus.Manager
	kubeClient  kubernetes.Interface
	kubeContext string
	runtime     string
}

var runCfg RunConfig

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run A/B comparison tests",
	Long: `Run load tests comparing old and new versions of Reloader.

Examples:
  # Compare two images
  loadtest run --old-image=stakater/reloader:v1.0.0 --new-image=stakater/reloader:v1.1.0

  # Run specific scenario
  loadtest run --old-image=stakater/reloader:v1.0.0 --new-image=localhost/reloader:dev --scenario=S2

  # Test single image (no comparison)
  loadtest run --new-image=localhost/reloader:test

  # Run all scenarios in parallel on 4 clusters
  loadtest run --new-image=localhost/reloader:test --parallelism=4`,
	Run: func(cmd *cobra.Command, args []string) {
		runCommand()
	},
}

func init() {
	runCmd.Flags().StringVar(&runCfg.OldImage, "old-image", "", "Container image for \"old\" version (required for comparison)")
	runCmd.Flags().StringVar(&runCfg.NewImage, "new-image", "", "Container image for \"new\" version (required for comparison)")
	runCmd.Flags().StringVar(&runCfg.Scenario, "scenario", "all", "Test scenario: S1-S13 or \"all\"")
	runCmd.Flags().IntVar(&runCfg.Duration, "duration", 60, "Test duration in seconds")
	runCmd.Flags().IntVar(&runCfg.Parallelism, "parallelism", 1, "Run N scenarios in parallel on N clusters")
	runCmd.Flags().BoolVar(&runCfg.SkipCluster, "skip-cluster", false, "Skip kind cluster creation (use existing)")
	runCmd.Flags().StringVar(&runCfg.ClusterName, "cluster-name", DefaultClusterName, "Kind cluster name")
	runCmd.Flags().StringVar(&runCfg.ResultsDir, "results-dir", "./results", "Directory for results")
	runCmd.Flags().StringVar(&runCfg.ManifestsDir, "manifests-dir", "", "Directory containing manifests (auto-detected if not set)")
}

func runCommand() {
	if runCfg.ManifestsDir == "" {
		execPath, _ := os.Executable()
		execDir := filepath.Dir(execPath)
		runCfg.ManifestsDir = filepath.Join(execDir, "..", "..", "manifests")
		if _, err := os.Stat(runCfg.ManifestsDir); os.IsNotExist(err) {
			runCfg.ManifestsDir = "./manifests"
		}
	}

	if runCfg.Parallelism < 1 {
		runCfg.Parallelism = 1
	}

	if runCfg.OldImage == "" && runCfg.NewImage == "" {
		log.Fatal("At least one of --old-image or --new-image is required")
	}

	runOld := runCfg.OldImage != ""
	runNew := runCfg.NewImage != ""
	runBoth := runOld && runNew

	log.Printf("Configuration:")
	log.Printf("  Scenario: %s", runCfg.Scenario)
	log.Printf("  Duration: %ds", runCfg.Duration)
	log.Printf("  Parallelism: %d", runCfg.Parallelism)
	if runCfg.OldImage != "" {
		log.Printf("  Old image: %s", runCfg.OldImage)
	}
	if runCfg.NewImage != "" {
		log.Printf("  New image: %s", runCfg.NewImage)
	}

	runtime, err := cluster.DetectContainerRuntime()
	if err != nil {
		log.Fatalf("Failed to detect container runtime: %v", err)
	}
	log.Printf("  Container runtime: %s", runtime)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal...")
		cancel()
	}()

	scenariosToRun := []string{runCfg.Scenario}
	if runCfg.Scenario == "all" {
		scenariosToRun = []string{"S1", "S2", "S3", "S4", "S5", "S6", "S7", "S8", "S9", "S10", "S11", "S12", "S13"}
	}

	if runCfg.SkipCluster && runCfg.Parallelism > 1 {
		log.Fatal("--skip-cluster is not supported with --parallelism > 1")
	}

	if runCfg.Parallelism > 1 {
		runParallel(ctx, runCfg, scenariosToRun, runtime, runOld, runNew, runBoth)
		return
	}

	runSequential(ctx, runCfg, scenariosToRun, runtime, runOld, runNew, runBoth)
}

func runSequential(ctx context.Context, cfg RunConfig, scenariosToRun []string, runtime string, runOld, runNew, runBoth bool) {
	clusterMgr := cluster.NewManager(cluster.Config{
		Name:             cfg.ClusterName,
		ContainerRuntime: runtime,
	})

	if cfg.SkipCluster {
		log.Printf("Skipping cluster creation (using existing cluster: %s)", cfg.ClusterName)
		if !clusterMgr.Exists() {
			log.Fatalf("Cluster %s does not exist. Remove --skip-cluster to create it.", cfg.ClusterName)
		}
	} else {
		log.Println("Creating kind cluster...")
		if err := clusterMgr.Create(ctx); err != nil {
			log.Fatalf("Failed to create cluster: %v", err)
		}
	}

	promManifest := filepath.Join(cfg.ManifestsDir, "prometheus.yaml")
	promMgr := prometheus.NewManager(promManifest)

	log.Println("Installing Prometheus...")
	if err := promMgr.Deploy(ctx); err != nil {
		log.Fatalf("Failed to deploy Prometheus: %v", err)
	}

	if err := promMgr.StartPortForward(ctx); err != nil {
		log.Fatalf("Failed to start Prometheus port-forward: %v", err)
	}
	defer promMgr.StopPortForward()

	log.Println("Loading images into kind cluster...")
	if runOld {
		log.Printf("Loading old image: %s", cfg.OldImage)
		if err := clusterMgr.LoadImage(ctx, cfg.OldImage); err != nil {
			log.Fatalf("Failed to load old image: %v", err)
		}
	}
	if runNew {
		log.Printf("Loading new image: %s", cfg.NewImage)
		if err := clusterMgr.LoadImage(ctx, cfg.NewImage); err != nil {
			log.Fatalf("Failed to load new image: %v", err)
		}
	}

	log.Println("Pre-loading test images...")
	testImage := "gcr.io/google-containers/busybox:1.27"
	clusterMgr.LoadImage(ctx, testImage)

	kubeClient, err := getKubeClient("")
	if err != nil {
		log.Fatalf("Failed to create kubernetes client: %v", err)
	}

	for _, scenarioID := range scenariosToRun {
		log.Printf("========================================")
		log.Printf("=== Starting scenario %s ===", scenarioID)
		log.Printf("========================================")

		cleanupTestNamespaces(ctx, "")
		cleanupReloader(ctx, "old", "")
		cleanupReloader(ctx, "new", "")

		if err := promMgr.Reset(ctx); err != nil {
			log.Printf("Warning: failed to reset Prometheus: %v", err)
		}

		createTestNamespace(ctx, "")

		if runOld {
			oldMgr := reloader.NewManager(reloader.Config{
				Version: "old",
				Image:   cfg.OldImage,
			})

			if err := oldMgr.Deploy(ctx); err != nil {
				log.Printf("Failed to deploy old Reloader: %v", err)
				continue
			}

			if err := promMgr.WaitForTarget(ctx, oldMgr.Job(), 60*time.Second); err != nil {
				log.Printf("Warning: %v", err)
				log.Println("Proceeding anyway, but metrics may be incomplete")
			}

			runScenario(ctx, kubeClient, scenarioID, "old", cfg.OldImage, cfg.Duration, cfg.ResultsDir)
			collectMetrics(ctx, promMgr, oldMgr.Job(), scenarioID, "old", cfg.ResultsDir)
			collectLogs(ctx, oldMgr, scenarioID, "old", cfg.ResultsDir)

			if runBoth {
				cleanupTestNamespaces(ctx, "")
				oldMgr.Cleanup(ctx)
				promMgr.Reset(ctx)
				createTestNamespace(ctx, "")
			}
		}

		if runNew {
			newMgr := reloader.NewManager(reloader.Config{
				Version: "new",
				Image:   cfg.NewImage,
			})

			if err := newMgr.Deploy(ctx); err != nil {
				log.Printf("Failed to deploy new Reloader: %v", err)
				continue
			}

			if err := promMgr.WaitForTarget(ctx, newMgr.Job(), 60*time.Second); err != nil {
				log.Printf("Warning: %v", err)
				log.Println("Proceeding anyway, but metrics may be incomplete")
			}

			runScenario(ctx, kubeClient, scenarioID, "new", cfg.NewImage, cfg.Duration, cfg.ResultsDir)
			collectMetrics(ctx, promMgr, newMgr.Job(), scenarioID, "new", cfg.ResultsDir)
			collectLogs(ctx, newMgr, scenarioID, "new", cfg.ResultsDir)
		}

		generateReport(scenarioID, cfg.ResultsDir, runBoth)
		log.Printf("=== Scenario %s complete ===", scenarioID)
	}

	log.Println("Load test complete!")
	log.Printf("Results available in: %s", cfg.ResultsDir)
}

func runParallel(ctx context.Context, cfg RunConfig, scenariosToRun []string, runtime string, runOld, runNew, runBoth bool) {
	numWorkers := cfg.Parallelism
	if numWorkers > len(scenariosToRun) {
		numWorkers = len(scenariosToRun)
		log.Printf("Reducing parallelism to %d (number of scenarios)", numWorkers)
	}

	log.Printf("Starting parallel execution with %d workers", numWorkers)

	workers := make([]*workerContext, numWorkers)
	var setupWg sync.WaitGroup
	setupErrors := make(chan error, numWorkers)

	log.Println("Setting up worker clusters...")
	for i := range numWorkers {
		setupWg.Add(1)
		go func(workerID int) {
			defer setupWg.Done()
			worker, err := setupWorker(ctx, cfg, workerID, runtime, runOld, runNew)
			if err != nil {
				setupErrors <- fmt.Errorf("worker %d setup failed: %w", workerID, err)
				return
			}
			workers[workerID] = worker
		}(i)
	}

	setupWg.Wait()
	close(setupErrors)

	for err := range setupErrors {
		log.Printf("Error: %v", err)
	}

	readyWorkers := 0
	for _, w := range workers {
		if w != nil {
			readyWorkers++
		}
	}
	if readyWorkers == 0 {
		log.Fatal("No workers ready, aborting")
	}
	if readyWorkers < numWorkers {
		log.Printf("Warning: only %d/%d workers ready", readyWorkers, numWorkers)
	}

	defer func() {
		log.Println("Cleaning up worker clusters...")
		for _, w := range workers {
			if w != nil {
				w.promMgr.StopPortForward()
			}
		}
	}()

	scenarioCh := make(chan string, len(scenariosToRun))
	for _, s := range scenariosToRun {
		scenarioCh <- s
	}
	close(scenarioCh)

	var resultsMu sync.Mutex
	completedScenarios := make([]string, 0, len(scenariosToRun))

	var wg sync.WaitGroup
	for _, worker := range workers {
		if worker == nil {
			continue
		}
		wg.Add(1)
		go func(w *workerContext) {
			defer wg.Done()
			for scenarioID := range scenarioCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				log.Printf("[Worker %d] Starting scenario %s", w.id, scenarioID)

				cleanupTestNamespaces(ctx, w.kubeContext)
				cleanupReloader(ctx, "old", w.kubeContext)
				cleanupReloader(ctx, "new", w.kubeContext)

				if err := w.promMgr.Reset(ctx); err != nil {
					log.Printf("[Worker %d] Warning: failed to reset Prometheus: %v", w.id, err)
				}

				createTestNamespace(ctx, w.kubeContext)

				if runOld {
					runVersionOnWorker(ctx, w, cfg, scenarioID, "old", cfg.OldImage, runBoth)
				}

				if runNew {
					runVersionOnWorker(ctx, w, cfg, scenarioID, "new", cfg.NewImage, false)
				}

				generateReport(scenarioID, cfg.ResultsDir, runBoth)

				resultsMu.Lock()
				completedScenarios = append(completedScenarios, scenarioID)
				resultsMu.Unlock()

				log.Printf("[Worker %d] Scenario %s complete", w.id, scenarioID)
			}
		}(worker)
	}

	wg.Wait()

	log.Println("Load test complete!")
	log.Printf("Completed %d/%d scenarios", len(completedScenarios), len(scenariosToRun))
	log.Printf("Results available in: %s", cfg.ResultsDir)
}

func setupWorker(ctx context.Context, cfg RunConfig, workerID int, runtime string, runOld, runNew bool) (*workerContext, error) {
	workerName := fmt.Sprintf("%s-%d", DefaultClusterName, workerID)
	promPort := 9091 + workerID

	log.Printf("[Worker %d] Creating cluster %s (ports %d/%d)...", workerID, workerName, 8080+workerID, 8443+workerID)

	clusterMgr := cluster.NewManager(cluster.Config{
		Name:             workerName,
		ContainerRuntime: runtime,
		PortOffset:       workerID,
	})

	if err := clusterMgr.Create(ctx); err != nil {
		return nil, fmt.Errorf("creating cluster: %w", err)
	}

	kubeContext := clusterMgr.Context()

	promManifest := filepath.Join(cfg.ManifestsDir, "prometheus.yaml")
	promMgr := prometheus.NewManagerWithPort(promManifest, promPort, kubeContext)

	log.Printf("[Worker %d] Installing Prometheus (port %d)...", workerID, promPort)
	if err := promMgr.Deploy(ctx); err != nil {
		return nil, fmt.Errorf("deploying prometheus: %w", err)
	}

	if err := promMgr.StartPortForward(ctx); err != nil {
		return nil, fmt.Errorf("starting prometheus port-forward: %w", err)
	}

	log.Printf("[Worker %d] Loading images...", workerID)
	if runOld {
		if err := clusterMgr.LoadImage(ctx, cfg.OldImage); err != nil {
			log.Printf("[Worker %d] Warning: failed to load old image: %v", workerID, err)
		}
	}
	if runNew {
		if err := clusterMgr.LoadImage(ctx, cfg.NewImage); err != nil {
			log.Printf("[Worker %d] Warning: failed to load new image: %v", workerID, err)
		}
	}

	testImage := "gcr.io/google-containers/busybox:1.27"
	clusterMgr.LoadImage(ctx, testImage)

	kubeClient, err := getKubeClient(kubeContext)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	log.Printf("[Worker %d] Ready", workerID)
	return &workerContext{
		id:          workerID,
		clusterMgr:  clusterMgr,
		promMgr:     promMgr,
		kubeClient:  kubeClient,
		kubeContext: kubeContext,
		runtime:     runtime,
	}, nil
}

func runVersionOnWorker(ctx context.Context, w *workerContext, cfg RunConfig, scenarioID, version, image string, cleanupAfter bool) {
	mgr := reloader.NewManager(reloader.Config{
		Version: version,
		Image:   image,
	})
	mgr.SetKubeContext(w.kubeContext)

	if err := mgr.Deploy(ctx); err != nil {
		log.Printf("[Worker %d] Failed to deploy %s Reloader: %v", w.id, version, err)
		return
	}

	if err := w.promMgr.WaitForTarget(ctx, mgr.Job(), 60*time.Second); err != nil {
		log.Printf("[Worker %d] Warning: %v", w.id, err)
		log.Printf("[Worker %d] Proceeding anyway, but metrics may be incomplete", w.id)
	}

	runScenario(ctx, w.kubeClient, scenarioID, version, image, cfg.Duration, cfg.ResultsDir)
	collectMetrics(ctx, w.promMgr, mgr.Job(), scenarioID, version, cfg.ResultsDir)
	collectLogs(ctx, mgr, scenarioID, version, cfg.ResultsDir)

	if cleanupAfter {
		cleanupTestNamespaces(ctx, w.kubeContext)
		mgr.Cleanup(ctx)
		w.promMgr.Reset(ctx)
		createTestNamespace(ctx, w.kubeContext)
	}
}

func runScenario(ctx context.Context, client kubernetes.Interface, scenarioID, version, image string, duration int, resultsDir string) {
	runner, ok := scenarios.Registry[scenarioID]
	if !ok {
		log.Printf("Unknown scenario: %s", scenarioID)
		return
	}

	if s6, ok := runner.(*scenarios.ControllerRestartScenario); ok {
		s6.ReloaderVersion = version
	}

	if s11, ok := runner.(*scenarios.AnnotationStrategyScenario); ok {
		s11.Image = image
	}

	log.Printf("Running scenario %s (%s): %s", scenarioID, version, runner.Description())

	if ctx.Err() != nil {
		log.Printf("WARNING: Parent context already done: %v", ctx.Err())
	}

	timeout := time.Duration(duration)*time.Second + 5*time.Minute
	log.Printf("Creating scenario context with timeout: %v (duration=%ds)", timeout, duration)

	scenarioCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	expected, err := runner.Run(scenarioCtx, client, TestNamespace, time.Duration(duration)*time.Second)
	if err != nil {
		log.Printf("Scenario %s failed: %v", scenarioID, err)
	}

	scenarios.WriteExpectedMetrics(scenarioID, resultsDir, expected)
}

func collectMetrics(ctx context.Context, promMgr *prometheus.Manager, job, scenarioID, version, resultsDir string) {
	log.Printf("Waiting 5s for Reloader to finish processing events...")
	time.Sleep(5 * time.Second)

	log.Printf("Waiting 8s for Prometheus to scrape final metrics...")
	time.Sleep(8 * time.Second)

	log.Printf("Collecting metrics for %s...", version)
	outputDir := filepath.Join(resultsDir, scenarioID, version)
	if err := promMgr.CollectMetrics(ctx, job, outputDir, scenarioID); err != nil {
		log.Printf("Failed to collect metrics: %v", err)
	}
}

func collectLogs(ctx context.Context, mgr *reloader.Manager, scenarioID, version, resultsDir string) {
	log.Printf("Collecting logs for %s...", version)
	logPath := filepath.Join(resultsDir, scenarioID, version, "reloader.log")
	if err := mgr.CollectLogs(ctx, logPath); err != nil {
		log.Printf("Failed to collect logs: %v", err)
	}
}

func generateReport(scenarioID, resultsDir string, isComparison bool) {
	if isComparison {
		log.Println("Generating comparison report...")
	} else {
		log.Println("Generating single-version report...")
	}

	reportPath := filepath.Join(resultsDir, scenarioID, "report.txt")

	cmd := exec.Command(os.Args[0], "report",
		fmt.Sprintf("--scenario=%s", scenarioID),
		fmt.Sprintf("--results-dir=%s", resultsDir),
		fmt.Sprintf("--output=%s", reportPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	if data, err := os.ReadFile(reportPath); err == nil {
		fmt.Println(string(data))
	}

	log.Printf("Report saved to: %s", reportPath)
}

func getKubeClient(kubeContext string) (kubernetes.Interface, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	configOverrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		configOverrides.CurrentContext = kubeContext
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func createTestNamespace(ctx context.Context, kubeContext string) {
	args := []string{"create", "namespace", TestNamespace, "--dry-run=client", "-o", "yaml"}
	if kubeContext != "" {
		args = append([]string{"--context", kubeContext}, args...)
	}
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	out, _ := cmd.Output()

	applyArgs := []string{"apply", "-f", "-"}
	if kubeContext != "" {
		applyArgs = append([]string{"--context", kubeContext}, applyArgs...)
	}
	applyCmd := exec.CommandContext(ctx, "kubectl", applyArgs...)
	applyCmd.Stdin = strings.NewReader(string(out))
	applyCmd.Run()
}

func cleanupTestNamespaces(ctx context.Context, kubeContext string) {
	log.Println("Cleaning up test resources...")

	namespaces := []string{TestNamespace}
	for i := range 10 {
		namespaces = append(namespaces, fmt.Sprintf("%s-%d", TestNamespace, i))
	}

	for _, ns := range namespaces {
		args := []string{"delete", "namespace", ns, "--wait=false", "--ignore-not-found"}
		if kubeContext != "" {
			args = append([]string{"--context", kubeContext}, args...)
		}
		exec.CommandContext(ctx, "kubectl", args...).Run()
	}

	time.Sleep(2 * time.Second)

	for _, ns := range namespaces {
		args := []string{"delete", "pods", "--all", "-n", ns, "--grace-period=0", "--force"}
		if kubeContext != "" {
			args = append([]string{"--context", kubeContext}, args...)
		}
		exec.CommandContext(ctx, "kubectl", args...).Run()
	}
}

func cleanupReloader(ctx context.Context, version string, kubeContext string) {
	ns := fmt.Sprintf("reloader-%s", version)

	nsArgs := []string{"delete", "namespace", ns, "--wait=false", "--ignore-not-found"}
	crArgs := []string{"delete", "clusterrole", fmt.Sprintf("reloader-%s", version), "--ignore-not-found"}
	crbArgs := []string{"delete", "clusterrolebinding", fmt.Sprintf("reloader-%s", version), "--ignore-not-found"}

	if kubeContext != "" {
		nsArgs = append([]string{"--context", kubeContext}, nsArgs...)
		crArgs = append([]string{"--context", kubeContext}, crArgs...)
		crbArgs = append([]string{"--context", kubeContext}, crbArgs...)
	}

	exec.CommandContext(ctx, "kubectl", nsArgs...).Run()
	exec.CommandContext(ctx, "kubectl", crArgs...).Run()
	exec.CommandContext(ctx, "kubectl", crbArgs...).Run()
}
