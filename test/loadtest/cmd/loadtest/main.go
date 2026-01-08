// Package main implements the unified load test CLI for Reloader A/B comparison.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/stakater/Reloader/test/loadtest/internal/cluster"
	"github.com/stakater/Reloader/test/loadtest/internal/prometheus"
	"github.com/stakater/Reloader/test/loadtest/internal/reloader"
	"github.com/stakater/Reloader/test/loadtest/internal/scenarios"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	clusterName   = "reloader-loadtest"
	testNamespace = "reloader-test"
)

// workerContext holds all resources for a single worker (cluster + prometheus).
type workerContext struct {
	id          int
	clusterMgr  *cluster.Manager
	promMgr     *prometheus.Manager
	kubeClient  kubernetes.Interface
	kubeContext string
	runtime     string
}

// Config holds CLI configuration.
type Config struct {
	OldImage     string
	NewImage     string
	Scenario     string
	Duration     int
	SkipCluster  bool
	ResultsDir   string
	ManifestsDir string
	Parallelism  int // Number of parallel clusters (1 = sequential)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "run":
		runCommand(os.Args[2:])
	case "report":
		reportCommand(os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`Reloader Load Test CLI

Usage:
  loadtest run [options]     Run A/B comparison tests
  loadtest report [options]  Generate comparison report
  loadtest help              Show this help

Run Options:
  --old-image=IMAGE     Container image for "old" version (required for comparison)
  --new-image=IMAGE     Container image for "new" version (required for comparison)
  --scenario=ID         Test scenario: S1-S13 or "all" (default: all)
  --duration=SECONDS    Test duration in seconds (default: 60)
  --parallelism=N       Run N scenarios in parallel on N clusters (default: 1)
  --skip-cluster        Skip kind cluster creation (use existing, only for parallelism=1)
  --results-dir=DIR     Directory for results (default: ./results)

Report Options:
  --scenario=ID         Scenario to report on (required)
  --results-dir=DIR     Directory containing results (default: ./results)
  --output=FILE         Output file (default: stdout)

Examples:
  # Compare two images
  loadtest run --old-image=stakater/reloader:v1.0.0 --new-image=stakater/reloader:v1.1.0

  # Run specific scenario
  loadtest run --old-image=stakater/reloader:v1.0.0 --new-image=localhost/reloader:dev --scenario=S2

  # Test single image (no comparison)
  loadtest run --new-image=localhost/reloader:test

  # Run all scenarios in parallel on 4 clusters
  loadtest run --new-image=localhost/reloader:test --parallelism=4

  # Run all 13 scenarios in parallel (one cluster per scenario)
  loadtest run --new-image=localhost/reloader:test --parallelism=13

  # Generate report
  loadtest report --scenario=S2 --results-dir=./results
`)
}

func parseArgs(args []string) Config {
	cfg := Config{
		Scenario:    "all",
		Duration:    60,
		ResultsDir:  "./results",
		Parallelism: 1,
	}

	// Find manifests dir relative to executable or current dir
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	cfg.ManifestsDir = filepath.Join(execDir, "..", "..", "manifests")
	if _, err := os.Stat(cfg.ManifestsDir); os.IsNotExist(err) {
		// Try relative to current dir
		cfg.ManifestsDir = "./manifests"
	}

	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--old-image="):
			cfg.OldImage = strings.TrimPrefix(arg, "--old-image=")
		case strings.HasPrefix(arg, "--new-image="):
			cfg.NewImage = strings.TrimPrefix(arg, "--new-image=")
		case strings.HasPrefix(arg, "--scenario="):
			cfg.Scenario = strings.TrimPrefix(arg, "--scenario=")
		case strings.HasPrefix(arg, "--duration="):
			if n, _ := fmt.Sscanf(strings.TrimPrefix(arg, "--duration="), "%d", &cfg.Duration); n != 1 {
				log.Printf("Warning: invalid --duration value, using default (%d)", cfg.Duration)
			}
		case strings.HasPrefix(arg, "--parallelism="):
			if n, _ := fmt.Sscanf(strings.TrimPrefix(arg, "--parallelism="), "%d", &cfg.Parallelism); n != 1 {
				log.Printf("Warning: invalid --parallelism value, using default (%d)", cfg.Parallelism)
			}
		case arg == "--skip-cluster":
			cfg.SkipCluster = true
		case strings.HasPrefix(arg, "--results-dir="):
			cfg.ResultsDir = strings.TrimPrefix(arg, "--results-dir=")
		case strings.HasPrefix(arg, "--manifests-dir="):
			cfg.ManifestsDir = strings.TrimPrefix(arg, "--manifests-dir=")
		}
	}

	// Validate parallelism
	if cfg.Parallelism < 1 {
		cfg.Parallelism = 1
	}

	return cfg
}

func runCommand(args []string) {
	cfg := parseArgs(args)

	// Validate required args
	if cfg.OldImage == "" && cfg.NewImage == "" {
		log.Fatal("At least one of --old-image or --new-image is required")
	}

	// Determine mode
	runOld := cfg.OldImage != ""
	runNew := cfg.NewImage != ""
	runBoth := runOld && runNew

	log.Printf("Configuration:")
	log.Printf("  Scenario: %s", cfg.Scenario)
	log.Printf("  Duration: %ds", cfg.Duration)
	log.Printf("  Parallelism: %d", cfg.Parallelism)
	if cfg.OldImage != "" {
		log.Printf("  Old image: %s", cfg.OldImage)
	}
	if cfg.NewImage != "" {
		log.Printf("  New image: %s", cfg.NewImage)
	}

	// Detect container runtime
	runtime, err := cluster.DetectContainerRuntime()
	if err != nil {
		log.Fatalf("Failed to detect container runtime: %v", err)
	}
	log.Printf("  Container runtime: %s", runtime)

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal...")
		cancel()
	}()

	// Determine scenarios to run
	scenariosToRun := []string{cfg.Scenario}
	if cfg.Scenario == "all" {
		scenariosToRun = []string{"S1", "S2", "S3", "S4", "S5", "S6", "S7", "S8", "S9", "S10", "S11", "S12", "S13"}
	}

	// Skip-cluster only works for parallelism=1
	if cfg.SkipCluster && cfg.Parallelism > 1 {
		log.Fatal("--skip-cluster is not supported with --parallelism > 1")
	}

	// If parallelism > 1, use parallel execution
	if cfg.Parallelism > 1 {
		runParallel(ctx, cfg, scenariosToRun, runtime, runOld, runNew, runBoth)
		return
	}

	// Sequential execution (parallelism == 1)
	runSequential(ctx, cfg, scenariosToRun, runtime, runOld, runNew, runBoth)
}

// runSequential runs scenarios one by one on a single cluster.
func runSequential(ctx context.Context, cfg Config, scenariosToRun []string, runtime string, runOld, runNew, runBoth bool) {
	// Create cluster manager
	clusterMgr := cluster.NewManager(cluster.Config{
		Name:             clusterName,
		ContainerRuntime: runtime,
	})

	// Create/verify cluster
	if cfg.SkipCluster {
		log.Println("Skipping cluster creation (using existing)")
		if !clusterMgr.Exists() {
			log.Fatalf("Cluster %s does not exist. Remove --skip-cluster to create it.", clusterName)
		}
	} else {
		log.Println("Creating kind cluster...")
		if err := clusterMgr.Create(ctx); err != nil {
			log.Fatalf("Failed to create cluster: %v", err)
		}
	}

	// Deploy Prometheus
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

	// Load images into kind
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

	// Pre-pull test images
	log.Println("Pre-loading test images...")
	testImage := "gcr.io/google-containers/busybox:1.27"
	clusterMgr.LoadImage(ctx, testImage) // Ignore errors

	// Get kubernetes client
	kubeClient, err := getKubeClient("")
	if err != nil {
		log.Fatalf("Failed to create kubernetes client: %v", err)
	}

	for _, scenarioID := range scenariosToRun {
		log.Printf("========================================")
		log.Printf("=== Starting scenario %s ===", scenarioID)
		log.Printf("========================================")

		// Clean up from previous scenario
		cleanupTestNamespaces(ctx, "")
		cleanupReloader(ctx, "old", "")
		cleanupReloader(ctx, "new", "")

		// Reset Prometheus
		if err := promMgr.Reset(ctx); err != nil {
			log.Printf("Warning: failed to reset Prometheus: %v", err)
		}

		// Create test namespace
		createTestNamespace(ctx, "")

		if runOld {
			// Test old version
			oldMgr := reloader.NewManager(reloader.Config{
				Version: "old",
				Image:   cfg.OldImage,
			})

			if err := oldMgr.Deploy(ctx); err != nil {
				log.Printf("Failed to deploy old Reloader: %v", err)
				continue
			}

			// Wait for Prometheus to discover and scrape the Reloader
			if err := promMgr.WaitForTarget(ctx, oldMgr.Job(), 60*time.Second); err != nil {
				log.Printf("Warning: %v", err)
				log.Println("Proceeding anyway, but metrics may be incomplete")
			}

			runScenario(ctx, kubeClient, scenarioID, "old", cfg.OldImage, cfg.Duration, cfg.ResultsDir)
			collectMetrics(ctx, promMgr, oldMgr.Job(), scenarioID, "old", cfg.ResultsDir)
			collectLogs(ctx, oldMgr, scenarioID, "old", cfg.ResultsDir)

			if runBoth {
				// Clean up for new version
				cleanupTestNamespaces(ctx, "")
				oldMgr.Cleanup(ctx)
				promMgr.Reset(ctx)
				createTestNamespace(ctx, "")
			}
		}

		if runNew {
			// Test new version
			newMgr := reloader.NewManager(reloader.Config{
				Version: "new",
				Image:   cfg.NewImage,
			})

			if err := newMgr.Deploy(ctx); err != nil {
				log.Printf("Failed to deploy new Reloader: %v", err)
				continue
			}

			// Wait for Prometheus to discover and scrape the Reloader
			if err := promMgr.WaitForTarget(ctx, newMgr.Job(), 60*time.Second); err != nil {
				log.Printf("Warning: %v", err)
				log.Println("Proceeding anyway, but metrics may be incomplete")
			}

			runScenario(ctx, kubeClient, scenarioID, "new", cfg.NewImage, cfg.Duration, cfg.ResultsDir)
			collectMetrics(ctx, promMgr, newMgr.Job(), scenarioID, "new", cfg.ResultsDir)
			collectLogs(ctx, newMgr, scenarioID, "new", cfg.ResultsDir)
		}

		// Generate report
		generateReport(scenarioID, cfg.ResultsDir, runBoth)

		log.Printf("=== Scenario %s complete ===", scenarioID)
	}

	log.Println("Load test complete!")
	log.Printf("Results available in: %s", cfg.ResultsDir)
}

// runParallel runs scenarios in parallel on N separate kind clusters.
func runParallel(ctx context.Context, cfg Config, scenariosToRun []string, runtime string, runOld, runNew, runBoth bool) {
	numWorkers := cfg.Parallelism
	if numWorkers > len(scenariosToRun) {
		numWorkers = len(scenariosToRun)
		log.Printf("Reducing parallelism to %d (number of scenarios)", numWorkers)
	}

	log.Printf("Starting parallel execution with %d workers", numWorkers)

	// Create workers
	workers := make([]*workerContext, numWorkers)
	var setupWg sync.WaitGroup
	setupErrors := make(chan error, numWorkers)

	log.Println("Setting up worker clusters...")
	for i := 0; i < numWorkers; i++ {
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

	// Check for setup errors
	for err := range setupErrors {
		log.Printf("Error: %v", err)
	}

	// Verify all workers are ready
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

	// Cleanup workers on exit
	defer func() {
		log.Println("Cleaning up worker clusters...")
		for _, w := range workers {
			if w != nil {
				w.promMgr.StopPortForward()
			}
		}
	}()

	// Create scenario channel
	scenarioCh := make(chan string, len(scenariosToRun))
	for _, s := range scenariosToRun {
		scenarioCh <- s
	}
	close(scenarioCh)

	// Results tracking
	var resultsMu sync.Mutex
	completedScenarios := make([]string, 0, len(scenariosToRun))

	// Start workers
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

				// Clean up from previous scenario
				cleanupTestNamespaces(ctx, w.kubeContext)
				cleanupReloader(ctx, "old", w.kubeContext)
				cleanupReloader(ctx, "new", w.kubeContext)

				// Reset Prometheus
				if err := w.promMgr.Reset(ctx); err != nil {
					log.Printf("[Worker %d] Warning: failed to reset Prometheus: %v", w.id, err)
				}

				// Create test namespace
				createTestNamespace(ctx, w.kubeContext)

				if runOld {
					runVersionOnWorker(ctx, w, cfg, scenarioID, "old", cfg.OldImage, runBoth)
				}

				if runNew {
					runVersionOnWorker(ctx, w, cfg, scenarioID, "new", cfg.NewImage, false)
				}

				// Generate report
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

// setupWorker creates a cluster and deploys prometheus for a single worker.
func setupWorker(ctx context.Context, cfg Config, workerID int, runtime string, runOld, runNew bool) (*workerContext, error) {
	workerName := fmt.Sprintf("%s-%d", clusterName, workerID)
	promPort := 9091 + workerID

	log.Printf("[Worker %d] Creating cluster %s (ports %d/%d)...", workerID, workerName, 8080+workerID, 8443+workerID)

	clusterMgr := cluster.NewManager(cluster.Config{
		Name:             workerName,
		ContainerRuntime: runtime,
		PortOffset:       workerID, // Each cluster gets unique ports
	})

	if err := clusterMgr.Create(ctx); err != nil {
		return nil, fmt.Errorf("creating cluster: %w", err)
	}

	kubeContext := clusterMgr.Context()

	// Deploy Prometheus
	promManifest := filepath.Join(cfg.ManifestsDir, "prometheus.yaml")
	promMgr := prometheus.NewManagerWithPort(promManifest, promPort, kubeContext)

	log.Printf("[Worker %d] Installing Prometheus (port %d)...", workerID, promPort)
	if err := promMgr.Deploy(ctx); err != nil {
		return nil, fmt.Errorf("deploying prometheus: %w", err)
	}

	if err := promMgr.StartPortForward(ctx); err != nil {
		return nil, fmt.Errorf("starting prometheus port-forward: %w", err)
	}

	// Load images
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

	// Pre-pull test images
	testImage := "gcr.io/google-containers/busybox:1.27"
	clusterMgr.LoadImage(ctx, testImage)

	// Get kubernetes client for this context
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

// runVersionOnWorker runs a single version test on a worker.
func runVersionOnWorker(ctx context.Context, w *workerContext, cfg Config, scenarioID, version, image string, cleanupAfter bool) {
	mgr := reloader.NewManager(reloader.Config{
		Version: version,
		Image:   image,
	})
	mgr.SetKubeContext(w.kubeContext)

	if err := mgr.Deploy(ctx); err != nil {
		log.Printf("[Worker %d] Failed to deploy %s Reloader: %v", w.id, version, err)
		return
	}

	// Wait for Prometheus to discover and scrape the Reloader
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

	// For S6, set the reloader version
	if s6, ok := runner.(*scenarios.ControllerRestartScenario); ok {
		s6.ReloaderVersion = version
	}

	// For S11, set the image to deploy its own Reloader
	if s11, ok := runner.(*scenarios.AnnotationStrategyScenario); ok {
		s11.Image = image
	}

	log.Printf("Running scenario %s (%s): %s", scenarioID, version, runner.Description())

	// Debug: check parent context state
	if ctx.Err() != nil {
		log.Printf("WARNING: Parent context already done: %v", ctx.Err())
	}

	// Add extra time for scenario setup (creating deployments, waiting for ready state)
	// Some scenarios like S2 create 50 deployments which can take 2-3 minutes
	timeout := time.Duration(duration)*time.Second + 5*time.Minute
	log.Printf("Creating scenario context with timeout: %v (duration=%ds)", timeout, duration)

	scenarioCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	expected, err := runner.Run(scenarioCtx, client, testNamespace, time.Duration(duration)*time.Second)
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

	// Use the report command
	cmd := exec.Command(os.Args[0], "report",
		fmt.Sprintf("--scenario=%s", scenarioID),
		fmt.Sprintf("--results-dir=%s", resultsDir),
		fmt.Sprintf("--output=%s", reportPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	// Also print to stdout
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
	args := []string{"create", "namespace", testNamespace, "--dry-run=client", "-o", "yaml"}
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

	// Main namespace + S3 extra namespaces
	namespaces := []string{testNamespace}
	for i := 0; i < 10; i++ {
		namespaces = append(namespaces, fmt.Sprintf("%s-%d", testNamespace, i))
	}

	for _, ns := range namespaces {
		args := []string{"delete", "namespace", ns, "--wait=false", "--ignore-not-found"}
		if kubeContext != "" {
			args = append([]string{"--context", kubeContext}, args...)
		}
		exec.CommandContext(ctx, "kubectl", args...).Run()
	}

	// Wait a bit for cleanup
	time.Sleep(2 * time.Second)

	// Force delete remaining pods
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

// ============================================================================
// REPORT COMMAND
// ============================================================================

func reportCommand(args []string) {
	var scenarioID, resultsDir, outputFile string
	resultsDir = "./results"

	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--scenario="):
			scenarioID = strings.TrimPrefix(arg, "--scenario=")
		case strings.HasPrefix(arg, "--results-dir="):
			resultsDir = strings.TrimPrefix(arg, "--results-dir=")
		case strings.HasPrefix(arg, "--output="):
			outputFile = strings.TrimPrefix(arg, "--output=")
		}
	}

	if scenarioID == "" {
		log.Fatal("--scenario is required for report command")
	}

	report, err := generateScenarioReport(scenarioID, resultsDir)
	if err != nil {
		log.Fatalf("Failed to generate report: %v", err)
	}

	output := renderScenarioReport(report)

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
		log.Printf("Report written to %s", outputFile)
	} else {
		fmt.Println(output)
	}
}

// PrometheusResponse represents a Prometheus API response for report parsing.
type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// MetricComparison represents the comparison of a single metric.
type MetricComparison struct {
	Name             string
	DisplayName      string
	Unit             string
	IsCounter        bool
	OldValue         float64
	NewValue         float64
	Expected         float64
	Difference       float64
	DiffPct          float64
	Status           string
	Threshold        float64
	OldMeetsExpected string
	NewMeetsExpected string
}

type metricInfo struct {
	unit      string
	isCounter bool
}

var metricInfoMap = map[string]metricInfo{
	"reconcile_total":              {unit: "count", isCounter: true},
	"reconcile_duration_p50":       {unit: "s", isCounter: false},
	"reconcile_duration_p95":       {unit: "s", isCounter: false},
	"reconcile_duration_p99":       {unit: "s", isCounter: false},
	"action_total":                 {unit: "count", isCounter: true},
	"action_latency_p50":           {unit: "s", isCounter: false},
	"action_latency_p95":           {unit: "s", isCounter: false},
	"action_latency_p99":           {unit: "s", isCounter: false},
	"errors_total":                 {unit: "count", isCounter: true},
	"reload_executed_total":        {unit: "count", isCounter: true},
	"workloads_scanned_total":      {unit: "count", isCounter: true},
	"workloads_matched_total":      {unit: "count", isCounter: true},
	"skipped_total_no_data_change": {unit: "count", isCounter: true},
	"rest_client_requests_total":   {unit: "count", isCounter: true},
	"rest_client_requests_get":     {unit: "count", isCounter: true},
	"rest_client_requests_patch":   {unit: "count", isCounter: true},
	"rest_client_requests_put":     {unit: "count", isCounter: true},
	"rest_client_requests_errors":  {unit: "count", isCounter: true},

	// Resource consumption metrics (gauges, not counters)
	"memory_rss_mb_avg":  {unit: "MB", isCounter: false},
	"memory_rss_mb_max":  {unit: "MB", isCounter: false},
	"memory_heap_mb_avg": {unit: "MB", isCounter: false},
	"memory_heap_mb_max": {unit: "MB", isCounter: false},
	"cpu_cores_avg":      {unit: "cores", isCounter: false},
	"cpu_cores_max":      {unit: "cores", isCounter: false},
	"goroutines_avg":     {unit: "count", isCounter: false},
	"goroutines_max":     {unit: "count", isCounter: false},
	"gc_pause_p99_ms":    {unit: "ms", isCounter: false},
}

// ReportExpectedMetrics matches the expected metrics from test scenarios.
type ReportExpectedMetrics struct {
	ActionTotal           int    `json:"action_total"`
	ReloadExecutedTotal   int    `json:"reload_executed_total"`
	ReconcileTotal        int    `json:"reconcile_total"`
	WorkloadsScannedTotal int    `json:"workloads_scanned_total"`
	WorkloadsMatchedTotal int    `json:"workloads_matched_total"`
	SkippedTotal          int    `json:"skipped_total"`
	Description           string `json:"description"`
}

// ScenarioReport represents the full report for a scenario.
type ScenarioReport struct {
	Scenario        string
	Timestamp       time.Time
	Comparisons     []MetricComparison
	OverallStatus   string
	Summary         string
	PassCriteria    []string
	FailedCriteria  []string
	Expected        ReportExpectedMetrics
	TestDescription string
}

// MetricType defines how to evaluate a metric.
type MetricType int

const (
	LowerIsBetter MetricType = iota
	ShouldMatch
	HigherIsBetter
	Informational // Reports values but doesn't affect pass/fail
)

type ThresholdConfig struct {
	maxDiff    float64
	metricType MetricType
	minAbsDiff float64
}

var thresholds = map[string]ThresholdConfig{
	"reconcile_total":              {maxDiff: 60.0, metricType: LowerIsBetter},
	"reconcile_duration_p50":       {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 0.5},
	"reconcile_duration_p95":       {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 1.0},
	"reconcile_duration_p99":       {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 1.0},
	"action_latency_p50":           {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 0.5},
	"action_latency_p95":           {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 1.0},
	"action_latency_p99":           {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 1.0},
	"errors_total":                 {maxDiff: 0.0, metricType: LowerIsBetter},
	"action_total":                 {maxDiff: 15.0, metricType: ShouldMatch},
	"reload_executed_total":        {maxDiff: 15.0, metricType: ShouldMatch},
	"workloads_scanned_total":      {maxDiff: 15.0, metricType: ShouldMatch},
	"workloads_matched_total":      {maxDiff: 15.0, metricType: ShouldMatch},
	"skipped_total_no_data_change": {maxDiff: 20.0, metricType: ShouldMatch},
	// API metrics - use minAbsDiff to allow small differences
	"rest_client_requests_total":  {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 50},
	"rest_client_requests_get":    {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 50},
	"rest_client_requests_patch":  {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 50},
	"rest_client_requests_put":    {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 20},
	"rest_client_requests_errors": {maxDiff: 0.0, metricType: LowerIsBetter, minAbsDiff: 100}, // Pass if both < 100

	// Resource consumption metrics
	"memory_rss_mb_avg":  {maxDiff: 50.0, metricType: LowerIsBetter, minAbsDiff: 20},   // 50% or 20MB
	"memory_rss_mb_max":  {maxDiff: 50.0, metricType: LowerIsBetter, minAbsDiff: 30},   // 50% or 30MB
	"memory_heap_mb_avg": {maxDiff: 50.0, metricType: LowerIsBetter, minAbsDiff: 15},   // 50% or 15MB
	"memory_heap_mb_max": {maxDiff: 50.0, metricType: LowerIsBetter, minAbsDiff: 20},   // 50% or 20MB
	"cpu_cores_avg":      {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 0.1}, // 100% or 0.1 cores
	"cpu_cores_max":      {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 0.2}, // 100% or 0.2 cores
	"goroutines_avg":     {metricType: Informational},                                  // Info only - different architectures may use more goroutines
	"goroutines_max":     {metricType: Informational},                                  // Info only - different architectures may use more goroutines
	"gc_pause_p99_ms":    {maxDiff: 100.0, metricType: LowerIsBetter, minAbsDiff: 5},   // 100% or 5ms
}

func generateScenarioReport(scenario, resultsDir string) (*ScenarioReport, error) {
	oldDir := filepath.Join(resultsDir, scenario, "old")
	newDir := filepath.Join(resultsDir, scenario, "new")
	scenarioDir := filepath.Join(resultsDir, scenario)

	// Check which directories exist to determine mode
	_, oldErr := os.Stat(oldDir)
	_, newErr := os.Stat(newDir)
	hasOld := oldErr == nil
	hasNew := newErr == nil
	isComparison := hasOld && hasNew

	// For single-version mode, determine which version we have
	singleVersion := ""
	singleDir := ""
	if !isComparison {
		if hasNew {
			singleVersion = "new"
			singleDir = newDir
		} else if hasOld {
			singleVersion = "old"
			singleDir = oldDir
		} else {
			return nil, fmt.Errorf("no results found in %s", scenarioDir)
		}
	}

	report := &ScenarioReport{
		Scenario:  scenario,
		Timestamp: time.Now(),
	}

	// Load expected metrics
	expectedPath := filepath.Join(scenarioDir, "expected.json")
	if data, err := os.ReadFile(expectedPath); err == nil {
		if err := json.Unmarshal(data, &report.Expected); err != nil {
			log.Printf("Warning: Could not parse expected metrics: %v", err)
		} else {
			report.TestDescription = report.Expected.Description
		}
	}

	// Handle single-version mode
	if !isComparison {
		return generateSingleVersionReport(report, singleDir, singleVersion, scenario)
	}

	// Define metrics to compare
	metricsToCompare := []struct {
		name     string
		file     string
		selector func(data PrometheusResponse) float64
	}{
		{"reconcile_total", "reloader_reconcile_total.json", sumAllValues},
		{"reconcile_duration_p50", "reconcile_p50.json", getFirstValue},
		{"reconcile_duration_p95", "reconcile_p95.json", getFirstValue},
		{"reconcile_duration_p99", "reconcile_p99.json", getFirstValue},
		{"action_total", "reloader_action_total.json", sumAllValues},
		{"action_latency_p50", "action_p50.json", getFirstValue},
		{"action_latency_p95", "action_p95.json", getFirstValue},
		{"action_latency_p99", "action_p99.json", getFirstValue},
		{"errors_total", "reloader_errors_total.json", sumAllValues},
		{"reload_executed_total", "reloader_reload_executed_total.json", sumSuccessValues},
		{"workloads_scanned_total", "reloader_workloads_scanned_total.json", sumAllValues},
		{"workloads_matched_total", "reloader_workloads_matched_total.json", sumAllValues},
		{"rest_client_requests_total", "rest_client_requests_total.json", getFirstValue},
		{"rest_client_requests_get", "rest_client_requests_get.json", getFirstValue},
		{"rest_client_requests_patch", "rest_client_requests_patch.json", getFirstValue},
		{"rest_client_requests_put", "rest_client_requests_put.json", getFirstValue},
		{"rest_client_requests_errors", "rest_client_requests_errors.json", getFirstValue},

		// Resource consumption metrics
		{"memory_rss_mb_avg", "memory_rss_bytes_avg.json", bytesToMB},
		{"memory_rss_mb_max", "memory_rss_bytes_max.json", bytesToMB},
		{"memory_heap_mb_avg", "memory_heap_bytes_avg.json", bytesToMB},
		{"memory_heap_mb_max", "memory_heap_bytes_max.json", bytesToMB},
		{"cpu_cores_avg", "cpu_usage_cores_avg.json", getFirstValue},
		{"cpu_cores_max", "cpu_usage_cores_max.json", getFirstValue},
		{"goroutines_avg", "goroutines_avg.json", getFirstValue},
		{"goroutines_max", "goroutines_max.json", getFirstValue},
		{"gc_pause_p99_ms", "gc_duration_seconds_p99.json", secondsToMs},
	}

	// Build expected values map
	expectedValues := map[string]float64{
		"action_total":            float64(report.Expected.ActionTotal),
		"reload_executed_total":   float64(report.Expected.ReloadExecutedTotal),
		"reconcile_total":         float64(report.Expected.ReconcileTotal),
		"workloads_scanned_total": float64(report.Expected.WorkloadsScannedTotal),
		"workloads_matched_total": float64(report.Expected.WorkloadsMatchedTotal),
		"skipped_total":           float64(report.Expected.SkippedTotal),
	}

	// First pass: collect all metric values
	metricValues := make(map[string]struct{ old, new, expected float64 })

	for _, m := range metricsToCompare {
		oldData, err := loadMetricFile(filepath.Join(oldDir, m.file))
		if err != nil {
			log.Printf("Warning: Could not load old metric %s: %v", m.name, err)
			continue
		}

		newData, err := loadMetricFile(filepath.Join(newDir, m.file))
		if err != nil {
			log.Printf("Warning: Could not load new metric %s: %v", m.name, err)
			continue
		}

		oldValue := m.selector(oldData)
		newValue := m.selector(newData)
		expected := expectedValues[m.name]

		metricValues[m.name] = struct{ old, new, expected float64 }{oldValue, newValue, expected}
	}

	// Check context for smart pass/fail decisions
	newMeetsActionExpected := false
	newReconcileIsZero := false
	isChurnScenario := scenario == "S5" // Workload churn has special pass/fail rules
	if v, ok := metricValues["action_total"]; ok && v.expected > 0 {
		tolerance := v.expected * 0.15
		newMeetsActionExpected = math.Abs(v.new-v.expected) <= tolerance
	}
	if v, ok := metricValues["reconcile_total"]; ok {
		newReconcileIsZero = v.new == 0
	}

	// Second pass: generate comparisons with context awareness
	for _, m := range metricsToCompare {
		v, ok := metricValues[m.name]
		if !ok {
			continue
		}

		comparison := compareMetricWithExpected(m.name, v.old, v.new, v.expected)

		// Context-aware adjustments for API metrics
		if strings.HasPrefix(m.name, "rest_client_requests") {
			// If new correctly processed all expected reloads but old didn't,
			// higher API calls in new is expected (it's doing the work correctly)
			if newMeetsActionExpected && comparison.Status != "pass" {
				if oldMeets, ok := metricValues["action_total"]; ok {
					oldTolerance := oldMeets.expected * 0.15
					oldMissed := math.Abs(oldMeets.old-oldMeets.expected) > oldTolerance
					if oldMissed {
						comparison.Status = "pass"
					}
				}
			}
			// If new has 0 reconciles (no-op scenario), API differences are fine
			if newReconcileIsZero && comparison.Status != "pass" {
				comparison.Status = "pass"
			}
		}

		// S5 (Workload Churn) specific adjustments:
		// - "Not found" errors are expected when deployments are deleted during processing
		// - No expected values for throughput, so compare old vs new (should be similar)
		if isChurnScenario {
			if m.name == "errors_total" {
				// In churn scenarios, "not found" errors are expected when workloads
				// are deleted while Reloader is processing them. Allow up to 50 errors.
				if v.new < 50 && v.old < 50 {
					comparison.Status = "pass"
				} else if v.new <= v.old*1.5 {
					// Also pass if new has similar or fewer errors than old
					comparison.Status = "pass"
				}
			}
			if m.name == "action_total" || m.name == "reload_executed_total" {
				// No expected value for churn - compare old vs new
				// Both should be similar (within 20% of each other)
				if v.old > 0 {
					diff := math.Abs(v.new-v.old) / v.old * 100
					if diff <= 20 {
						comparison.Status = "pass"
					}
				} else if v.new > 0 {
					// Old is 0, new has value - that's fine
					comparison.Status = "pass"
				}
			}
		}

		report.Comparisons = append(report.Comparisons, comparison)

		if comparison.Status == "pass" {
			report.PassCriteria = append(report.PassCriteria, m.name)
		} else if comparison.Status == "fail" {
			report.FailedCriteria = append(report.FailedCriteria, m.name)
		}
	}

	// Determine overall status
	if len(report.FailedCriteria) == 0 {
		report.OverallStatus = "PASS"
		report.Summary = "All metrics within acceptable thresholds"
	} else {
		report.OverallStatus = "FAIL"
		report.Summary = fmt.Sprintf("%d metrics failed: %s",
			len(report.FailedCriteria),
			strings.Join(report.FailedCriteria, ", "))
	}

	return report, nil
}

// generateSingleVersionReport creates a report for a single version (no comparison).
func generateSingleVersionReport(report *ScenarioReport, dataDir, version, scenario string) (*ScenarioReport, error) {
	// Define metrics to collect
	metricsToCollect := []struct {
		name     string
		file     string
		selector func(data PrometheusResponse) float64
	}{
		{"reconcile_total", "reloader_reconcile_total.json", sumAllValues},
		{"reconcile_duration_p50", "reconcile_p50.json", getFirstValue},
		{"reconcile_duration_p95", "reconcile_p95.json", getFirstValue},
		{"reconcile_duration_p99", "reconcile_p99.json", getFirstValue},
		{"action_total", "reloader_action_total.json", sumAllValues},
		{"action_latency_p50", "action_p50.json", getFirstValue},
		{"action_latency_p95", "action_p95.json", getFirstValue},
		{"action_latency_p99", "action_p99.json", getFirstValue},
		{"errors_total", "reloader_errors_total.json", sumAllValues},
		{"reload_executed_total", "reloader_reload_executed_total.json", sumSuccessValues},
		{"workloads_scanned_total", "reloader_workloads_scanned_total.json", sumAllValues},
		{"workloads_matched_total", "reloader_workloads_matched_total.json", sumAllValues},
		{"rest_client_requests_total", "rest_client_requests_total.json", getFirstValue},
		{"rest_client_requests_get", "rest_client_requests_get.json", getFirstValue},
		{"rest_client_requests_patch", "rest_client_requests_patch.json", getFirstValue},
		{"rest_client_requests_put", "rest_client_requests_put.json", getFirstValue},
		{"rest_client_requests_errors", "rest_client_requests_errors.json", getFirstValue},
		{"memory_rss_mb_avg", "memory_rss_bytes_avg.json", bytesToMB},
		{"memory_rss_mb_max", "memory_rss_bytes_max.json", bytesToMB},
		{"memory_heap_mb_avg", "memory_heap_bytes_avg.json", bytesToMB},
		{"memory_heap_mb_max", "memory_heap_bytes_max.json", bytesToMB},
		{"cpu_cores_avg", "cpu_usage_cores_avg.json", getFirstValue},
		{"cpu_cores_max", "cpu_usage_cores_max.json", getFirstValue},
		{"goroutines_avg", "goroutines_avg.json", getFirstValue},
		{"goroutines_max", "goroutines_max.json", getFirstValue},
		{"gc_pause_p99_ms", "gc_duration_seconds_p99.json", secondsToMs},
	}

	// Build expected values map
	expectedValues := map[string]float64{
		"action_total":            float64(report.Expected.ActionTotal),
		"reload_executed_total":   float64(report.Expected.ReloadExecutedTotal),
		"reconcile_total":         float64(report.Expected.ReconcileTotal),
		"workloads_scanned_total": float64(report.Expected.WorkloadsScannedTotal),
		"workloads_matched_total": float64(report.Expected.WorkloadsMatchedTotal),
		"skipped_total":           float64(report.Expected.SkippedTotal),
	}

	for _, m := range metricsToCollect {
		data, err := loadMetricFile(filepath.Join(dataDir, m.file))
		if err != nil {
			log.Printf("Warning: Could not load metric %s: %v", m.name, err)
			continue
		}

		value := m.selector(data)
		expected := expectedValues[m.name]

		info := metricInfoMap[m.name]
		if info.unit == "" {
			info = metricInfo{unit: "count", isCounter: true}
		}

		displayName := m.name
		if info.unit != "count" {
			displayName = fmt.Sprintf("%s (%s)", m.name, info.unit)
		}

		// For single-version, put the value in NewValue column
		status := "info"
		meetsExp := "-"

		// Check against expected if available
		if expected > 0 {
			meetsExp = meetsExpected(value, expected)
			threshold, ok := thresholds[m.name]
			if ok && threshold.metricType == ShouldMatch {
				if meetsExp == "✓" {
					status = "pass"
					report.PassCriteria = append(report.PassCriteria, m.name)
				} else {
					status = "fail"
					report.FailedCriteria = append(report.FailedCriteria, m.name)
				}
			}
		}

		if info.isCounter {
			value = math.Round(value)
		}

		report.Comparisons = append(report.Comparisons, MetricComparison{
			Name:             m.name,
			DisplayName:      displayName,
			Unit:             info.unit,
			IsCounter:        info.isCounter,
			OldValue:         0, // No old value in single-version mode
			NewValue:         value,
			Expected:         expected,
			OldMeetsExpected: "-",
			NewMeetsExpected: meetsExp,
			Status:           status,
		})
	}

	if len(report.FailedCriteria) == 0 {
		report.OverallStatus = "PASS"
		report.Summary = fmt.Sprintf("Single-version test (%s) completed successfully", version)
	} else {
		report.OverallStatus = "FAIL"
		report.Summary = fmt.Sprintf("%d metrics failed: %s",
			len(report.FailedCriteria),
			strings.Join(report.FailedCriteria, ", "))
	}

	return report, nil
}

func loadMetricFile(path string) (PrometheusResponse, error) {
	var resp PrometheusResponse
	data, err := os.ReadFile(path)
	if err != nil {
		return resp, err
	}
	err = json.Unmarshal(data, &resp)
	return resp, err
}

func sumAllValues(data PrometheusResponse) float64 {
	var sum float64
	for _, result := range data.Data.Result {
		if len(result.Value) >= 2 {
			if v, ok := result.Value[1].(string); ok {
				var f float64
				fmt.Sscanf(v, "%f", &f)
				sum += f
			}
		}
	}
	return sum
}

func sumSuccessValues(data PrometheusResponse) float64 {
	var sum float64
	for _, result := range data.Data.Result {
		if result.Metric["success"] == "true" {
			if len(result.Value) >= 2 {
				if v, ok := result.Value[1].(string); ok {
					var f float64
					fmt.Sscanf(v, "%f", &f)
					sum += f
				}
			}
		}
	}
	return sum
}

func getFirstValue(data PrometheusResponse) float64 {
	if len(data.Data.Result) > 0 && len(data.Data.Result[0].Value) >= 2 {
		if v, ok := data.Data.Result[0].Value[1].(string); ok {
			var f float64
			fmt.Sscanf(v, "%f", &f)
			return f
		}
	}
	return 0
}

// bytesToMB converts bytes to megabytes.
func bytesToMB(data PrometheusResponse) float64 {
	bytes := getFirstValue(data)
	return bytes / (1024 * 1024)
}

// secondsToMs converts seconds to milliseconds.
func secondsToMs(data PrometheusResponse) float64 {
	seconds := getFirstValue(data)
	return seconds * 1000
}

func meetsExpected(value, expected float64) string {
	if expected == 0 {
		return "-"
	}
	tolerance := expected * 0.15
	if math.Abs(value-expected) <= tolerance {
		return "✓"
	}
	return "✗"
}

func compareMetricWithExpected(name string, oldValue, newValue, expected float64) MetricComparison {
	diff := newValue - oldValue
	absDiff := math.Abs(diff)
	var diffPct float64
	if oldValue != 0 {
		diffPct = (diff / oldValue) * 100
	} else if newValue != 0 {
		diffPct = 100
	}

	threshold, ok := thresholds[name]
	if !ok {
		threshold = ThresholdConfig{maxDiff: 10.0, metricType: ShouldMatch}
	}

	info := metricInfoMap[name]
	if info.unit == "" {
		info = metricInfo{unit: "count", isCounter: true}
	}
	displayName := name
	if info.unit != "count" {
		displayName = fmt.Sprintf("%s (%s)", name, info.unit)
	}

	if info.isCounter {
		oldValue = math.Round(oldValue)
		newValue = math.Round(newValue)
	}

	status := "pass"
	oldMeetsExp := meetsExpected(oldValue, expected)
	newMeetsExp := meetsExpected(newValue, expected)

	if expected > 0 && threshold.metricType == ShouldMatch {
		if newMeetsExp == "✗" {
			status = "fail"
		}
	} else {
		switch threshold.metricType {
		case LowerIsBetter:
			if threshold.minAbsDiff > 0 && absDiff < threshold.minAbsDiff {
				status = "pass"
			} else if diffPct > threshold.maxDiff {
				status = "fail"
			}
		case HigherIsBetter:
			if diffPct < -threshold.maxDiff {
				status = "fail"
			}
		case ShouldMatch:
			if math.Abs(diffPct) > threshold.maxDiff {
				status = "fail"
			}
		case Informational:
			status = "info"
		}
	}

	return MetricComparison{
		Name:             name,
		DisplayName:      displayName,
		Unit:             info.unit,
		IsCounter:        info.isCounter,
		Expected:         expected,
		OldMeetsExpected: oldMeetsExp,
		NewMeetsExpected: newMeetsExp,
		OldValue:         oldValue,
		NewValue:         newValue,
		Difference:       diff,
		DiffPct:          diffPct,
		Status:           status,
		Threshold:        threshold.maxDiff,
	}
}

func renderScenarioReport(report *ScenarioReport) string {
	var sb strings.Builder

	// Detect single-version mode by checking if all OldValues are 0
	isSingleVersion := true
	for _, c := range report.Comparisons {
		if c.OldValue != 0 {
			isSingleVersion = false
			break
		}
	}

	sb.WriteString("\n")
	sb.WriteString("================================================================================\n")
	if isSingleVersion {
		sb.WriteString("                        RELOADER TEST REPORT\n")
	} else {
		sb.WriteString("                     RELOADER A/B COMPARISON REPORT\n")
	}
	sb.WriteString("================================================================================\n\n")

	fmt.Fprintf(&sb, "Scenario:     %s\n", report.Scenario)
	fmt.Fprintf(&sb, "Generated:    %s\n", report.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "Status:       %s\n", report.OverallStatus)
	fmt.Fprintf(&sb, "Summary:      %s\n", report.Summary)

	if report.TestDescription != "" {
		fmt.Fprintf(&sb, "Test:         %s\n", report.TestDescription)
	}

	if report.Expected.ActionTotal > 0 {
		sb.WriteString("\n--------------------------------------------------------------------------------\n")
		sb.WriteString("                           EXPECTED VALUES\n")
		sb.WriteString("--------------------------------------------------------------------------------\n")
		fmt.Fprintf(&sb, "Expected Action Total:          %d\n", report.Expected.ActionTotal)
		fmt.Fprintf(&sb, "Expected Reload Executed Total: %d\n", report.Expected.ReloadExecutedTotal)
		if report.Expected.SkippedTotal > 0 {
			fmt.Fprintf(&sb, "Expected Skipped Total:         %d\n", report.Expected.SkippedTotal)
		}
	}

	sb.WriteString("\n--------------------------------------------------------------------------------\n")
	if isSingleVersion {
		sb.WriteString("                              METRICS\n")
	} else {
		sb.WriteString("                           METRIC COMPARISONS\n")
	}
	sb.WriteString("--------------------------------------------------------------------------------\n")

	if isSingleVersion {
		sb.WriteString("(✓ = meets expected value within 15%)\n\n")
		fmt.Fprintf(&sb, "%-32s %12s %10s %5s %8s\n",
			"Metric", "Value", "Expected", "Met?", "Status")
		fmt.Fprintf(&sb, "%-32s %12s %10s %5s %8s\n",
			"------", "-----", "--------", "----", "------")

		for _, c := range report.Comparisons {
			if c.IsCounter {
				if c.Expected > 0 {
					fmt.Fprintf(&sb, "%-32s %12.0f %10.0f %5s %8s\n",
						c.DisplayName, c.NewValue, c.Expected,
						c.NewMeetsExpected, c.Status)
				} else {
					fmt.Fprintf(&sb, "%-32s %12.0f %10s %5s %8s\n",
						c.DisplayName, c.NewValue, "-",
						c.NewMeetsExpected, c.Status)
				}
			} else {
				fmt.Fprintf(&sb, "%-32s %12.4f %10s %5s %8s\n",
					c.DisplayName, c.NewValue, "-",
					c.NewMeetsExpected, c.Status)
			}
		}
	} else {
		sb.WriteString("(Old✓/New✓ = meets expected value within 15%)\n\n")

		fmt.Fprintf(&sb, "%-32s %12s %12s %10s %5s %5s %8s\n",
			"Metric", "Old", "New", "Expected", "Old✓", "New✓", "Status")
		fmt.Fprintf(&sb, "%-32s %12s %12s %10s %5s %5s %8s\n",
			"------", "---", "---", "--------", "----", "----", "------")

		for _, c := range report.Comparisons {
			if c.IsCounter {
				if c.Expected > 0 {
					fmt.Fprintf(&sb, "%-32s %12.0f %12.0f %10.0f %5s %5s %8s\n",
						c.DisplayName, c.OldValue, c.NewValue, c.Expected,
						c.OldMeetsExpected, c.NewMeetsExpected, c.Status)
				} else {
					fmt.Fprintf(&sb, "%-32s %12.0f %12.0f %10s %5s %5s %8s\n",
						c.DisplayName, c.OldValue, c.NewValue, "-",
						c.OldMeetsExpected, c.NewMeetsExpected, c.Status)
				}
			} else {
				fmt.Fprintf(&sb, "%-32s %12.4f %12.4f %10s %5s %5s %8s\n",
					c.DisplayName, c.OldValue, c.NewValue, "-",
					c.OldMeetsExpected, c.NewMeetsExpected, c.Status)
			}
		}
	}

	sb.WriteString("\n--------------------------------------------------------------------------------\n")
	sb.WriteString("                           PASS/FAIL CRITERIA\n")
	sb.WriteString("--------------------------------------------------------------------------------\n\n")

	fmt.Fprintf(&sb, "Passed (%d):\n", len(report.PassCriteria))
	for _, p := range report.PassCriteria {
		fmt.Fprintf(&sb, "  ✓ %s\n", p)
	}

	if len(report.FailedCriteria) > 0 {
		fmt.Fprintf(&sb, "\nFailed (%d):\n", len(report.FailedCriteria))
		for _, f := range report.FailedCriteria {
			fmt.Fprintf(&sb, "  ✗ %s\n", f)
		}
	}

	sb.WriteString("\n--------------------------------------------------------------------------------\n")
	sb.WriteString("                           THRESHOLDS USED\n")
	sb.WriteString("--------------------------------------------------------------------------------\n\n")

	fmt.Fprintf(&sb, "%-35s %10s %15s %18s\n",
		"Metric", "Max Diff%", "Min Abs Diff", "Direction")
	fmt.Fprintf(&sb, "%-35s %10s %15s %18s\n",
		"------", "---------", "------------", "---------")

	// Sort threshold names
	var names []string
	for name := range thresholds {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		t := thresholds[name]
		var direction string
		switch t.metricType {
		case LowerIsBetter:
			direction = "lower is better"
		case HigherIsBetter:
			direction = "higher is better"
		case ShouldMatch:
			direction = "should match"
		case Informational:
			direction = "info only"
		}
		minAbsDiff := "-"
		if t.minAbsDiff > 0 {
			minAbsDiff = fmt.Sprintf("%.1fs", t.minAbsDiff)
		}
		fmt.Fprintf(&sb, "%-35s %9.1f%% %15s %18s\n",
			name, t.maxDiff, minAbsDiff, direction)
	}

	sb.WriteString("\n================================================================================\n")

	return sb.String()
}
