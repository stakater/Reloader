// Package scenarios contains all load test scenario implementations.
package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/stakater/Reloader/test/loadtest/internal/reloader"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

// ExpectedMetrics holds the expected values for metrics verification.
type ExpectedMetrics struct {
	ActionTotal           int    `json:"action_total"`
	ReloadExecutedTotal   int    `json:"reload_executed_total"`
	ReconcileTotal        int    `json:"reconcile_total"`
	WorkloadsScannedTotal int    `json:"workloads_scanned_total"`
	WorkloadsMatchedTotal int    `json:"workloads_matched_total"`
	SkippedTotal          int    `json:"skipped_total"`
	Description           string `json:"description"`
}

// Runner defines the interface for test scenarios.
type Runner interface {
	Name() string
	Description() string
	Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error)
}

// Registry holds all available test scenarios.
var Registry = map[string]Runner{
	"S1":  &BurstUpdateScenario{},
	"S2":  &FanOutScenario{},
	"S3":  &HighCardinalityScenario{},
	"S4":  &NoOpUpdateScenario{},
	"S5":  &WorkloadChurnScenario{},
	"S6":  &ControllerRestartScenario{},
	"S7":  &APIPressureScenario{},
	"S8":  &LargeObjectScenario{},
	"S9":  &MultiWorkloadTypeScenario{},
	"S10": &SecretsAndMixedScenario{},
	"S11": &AnnotationStrategyScenario{},
	"S12": &PauseResumeScenario{},
	"S13": &ComplexReferencesScenario{},
}

// WriteExpectedMetrics writes expected metrics to a JSON file.
func WriteExpectedMetrics(scenario, resultsDir string, expected ExpectedMetrics) error {
	if resultsDir == "" {
		return nil
	}

	dir := filepath.Join(resultsDir, scenario)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating results directory: %w", err)
	}

	data, err := json.MarshalIndent(expected, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling expected metrics: %w", err)
	}

	path := filepath.Join(dir, "expected.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing expected metrics: %w", err)
	}

	log.Printf("Expected metrics written to %s", path)
	return nil
}

// BurstUpdateScenario - Many ConfigMap/Secret updates in quick succession.
type BurstUpdateScenario struct{}

func (s *BurstUpdateScenario) Name() string        { return "S1" }
func (s *BurstUpdateScenario) Description() string { return "Burst ConfigMap/Secret updates" }

func (s *BurstUpdateScenario) Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error) {
	log.Println("S1: Creating base ConfigMaps and Deployments...")

	const numConfigMaps = 10
	const numDeployments = 10

	setupCtx := context.Background()

	for i := 0; i < numConfigMaps; i++ {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("burst-cm-%d", i),
				Namespace: namespace,
			},
			Data: map[string]string{"key": "initial-value"},
		}
		if _, err := client.CoreV1().ConfigMaps(namespace).Create(setupCtx, cm, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create ConfigMap %s: %v", cm.Name, err)
		}
	}

	for i := 0; i < numDeployments; i++ {
		deploy := createDeployment(fmt.Sprintf("burst-deploy-%d", i), namespace, fmt.Sprintf("burst-cm-%d", i))
		if _, err := client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create Deployment: %v", err)
		}
	}

	if err := waitForDeploymentsReady(setupCtx, client, namespace, 3*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}

	log.Println("S1: Starting burst updates...")

	updateCount := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	endTime := time.Now().Add(duration - 5*time.Second)
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			log.Printf("S1: Context cancelled, completed %d burst updates", updateCount)
			return ExpectedMetrics{
				ActionTotal:           updateCount,
				ReloadExecutedTotal:   updateCount,
				WorkloadsMatchedTotal: updateCount,
				Description:           fmt.Sprintf("S1: %d burst updates, each triggers 1 deployment reload", updateCount),
			}, nil
		case <-ticker.C:
			cmIndex := rand.Intn(numConfigMaps)
			cm, err := client.CoreV1().ConfigMaps(namespace).Get(setupCtx, fmt.Sprintf("burst-cm-%d", cmIndex), metav1.GetOptions{})
			if err != nil {
				continue
			}
			cm.Data["key"] = fmt.Sprintf("value-%d-%d", updateCount, time.Now().UnixNano())
			if _, err := client.CoreV1().ConfigMaps(namespace).Update(setupCtx, cm, metav1.UpdateOptions{}); err != nil {
				log.Printf("Failed to update ConfigMap: %v", err)
			} else {
				updateCount++
			}
		}
	}

	log.Printf("S1: Completed %d burst updates", updateCount)
	return ExpectedMetrics{
		ActionTotal:           updateCount,
		ReloadExecutedTotal:   updateCount,
		WorkloadsMatchedTotal: updateCount,
		Description:           fmt.Sprintf("S1: %d burst updates, each triggers 1 deployment reload", updateCount),
	}, nil
}

// FanOutScenario - One ConfigMap used by many workloads.
type FanOutScenario struct{}

func (s *FanOutScenario) Name() string        { return "S2" }
func (s *FanOutScenario) Description() string { return "Fan-out (one CM -> many workloads)" }

func (s *FanOutScenario) Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error) {
	log.Println("S2: Creating shared ConfigMap and multiple Deployments...")

	const numDeployments = 50
	setupCtx := context.Background()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-cm",
			Namespace: namespace,
		},
		Data: map[string]string{"config": "initial"},
	}
	if _, err := client.CoreV1().ConfigMaps(namespace).Create(setupCtx, cm, metav1.CreateOptions{}); err != nil {
		return ExpectedMetrics{}, fmt.Errorf("failed to create shared ConfigMap: %w", err)
	}

	for i := 0; i < numDeployments; i++ {
		deploy := createDeployment(fmt.Sprintf("fanout-deploy-%d", i), namespace, "shared-cm")
		if _, err := client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create Deployment %d: %v", i, err)
		}
	}

	if err := waitForDeploymentsReady(setupCtx, client, namespace, 5*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}

	log.Println("S2: Updating shared ConfigMap...")

	// Check context state before starting update loop
	if ctx.Err() != nil {
		log.Printf("S2: WARNING - Context already done before update loop: %v", ctx.Err())
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		log.Printf("S2: Context deadline in %v", remaining)
		if remaining < 10*time.Second {
			log.Printf("S2: WARNING - Very little time remaining on context!")
		}
	} else {
		log.Println("S2: Context has no deadline")
	}

	updateCount := 0
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	endTime := time.Now().Add(duration - 5*time.Second)
	log.Printf("S2: Will run updates for %v (duration=%v)", duration-5*time.Second, duration)

	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			expectedActions := updateCount * numDeployments
			log.Printf("S2: Context done (err=%v), completed %d fan-out updates", ctx.Err(), updateCount)
			return ExpectedMetrics{
				ActionTotal:           expectedActions,
				ReloadExecutedTotal:   expectedActions,
				WorkloadsScannedTotal: expectedActions,
				WorkloadsMatchedTotal: expectedActions,
				Description:           fmt.Sprintf("S2: %d updates × %d deployments = %d expected reloads", updateCount, numDeployments, expectedActions),
			}, nil
		case <-ticker.C:
			cm, err := client.CoreV1().ConfigMaps(namespace).Get(setupCtx, "shared-cm", metav1.GetOptions{})
			if err != nil {
				continue
			}
			cm.Data["config"] = fmt.Sprintf("update-%d", updateCount)
			if _, err := client.CoreV1().ConfigMaps(namespace).Update(setupCtx, cm, metav1.UpdateOptions{}); err != nil {
				log.Printf("Failed to update shared ConfigMap: %v", err)
			} else {
				updateCount++
				log.Printf("S2: Updated shared ConfigMap (should trigger %d reloads)", numDeployments)
			}
		}
	}

	expectedActions := updateCount * numDeployments
	log.Printf("S2: Completed %d fan-out updates, expected %d total actions", updateCount, expectedActions)
	return ExpectedMetrics{
		ActionTotal:           expectedActions,
		ReloadExecutedTotal:   expectedActions,
		WorkloadsScannedTotal: expectedActions,
		WorkloadsMatchedTotal: expectedActions,
		Description:           fmt.Sprintf("S2: %d updates × %d deployments = %d expected reloads", updateCount, numDeployments, expectedActions),
	}, nil
}

// HighCardinalityScenario - Many ConfigMaps/Secrets across many namespaces.
type HighCardinalityScenario struct{}

func (s *HighCardinalityScenario) Name() string { return "S3" }
func (s *HighCardinalityScenario) Description() string {
	return "High cardinality (many CMs, many namespaces)"
}

func (s *HighCardinalityScenario) Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error) {
	log.Println("S3: Creating high cardinality resources...")

	setupCtx := context.Background()

	namespaces := []string{namespace}
	for i := 0; i < 10; i++ {
		ns := fmt.Sprintf("%s-%d", namespace, i)
		if _, err := client.CoreV1().Namespaces().Create(setupCtx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		}, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create namespace %s: %v", ns, err)
		} else {
			namespaces = append(namespaces, ns)
		}
	}

	for _, ns := range namespaces {
		for i := 0; i < 20; i++ {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("hc-cm-%d", i),
					Namespace: ns,
				},
				Data: map[string]string{"key": "value"},
			}
			client.CoreV1().ConfigMaps(ns).Create(setupCtx, cm, metav1.CreateOptions{})
			deploy := createDeployment(fmt.Sprintf("hc-deploy-%d", i), ns, fmt.Sprintf("hc-cm-%d", i))
			client.AppsV1().Deployments(ns).Create(setupCtx, deploy, metav1.CreateOptions{})
		}
	}

	if err := waitForAllNamespacesReady(setupCtx, client, namespaces, 5*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}

	log.Println("S3: Starting random updates across namespaces...")

	updateDuration := duration - 5*time.Second
	if updateDuration < 30*time.Second {
		updateDuration = 30 * time.Second
	}

	updateCount := 0
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	updateCtx, updateCancel := context.WithTimeout(context.Background(), updateDuration)
	defer updateCancel()

	endTime := time.Now().Add(updateDuration)
	log.Printf("S3: Will run updates for %v (until %v)", updateDuration, endTime.Format("15:04:05"))

	for time.Now().Before(endTime) {
		select {
		case <-updateCtx.Done():
			log.Printf("S3: Completed %d high cardinality updates", updateCount)
			return ExpectedMetrics{
				ActionTotal:         updateCount,
				ReloadExecutedTotal: updateCount,
				Description:         fmt.Sprintf("S3: %d updates across %d namespaces", updateCount, len(namespaces)),
			}, nil
		case <-ticker.C:
			ns := namespaces[rand.Intn(len(namespaces))]
			cmIndex := rand.Intn(20)
			cm, err := client.CoreV1().ConfigMaps(ns).Get(setupCtx, fmt.Sprintf("hc-cm-%d", cmIndex), metav1.GetOptions{})
			if err != nil {
				continue
			}
			cm.Data["key"] = fmt.Sprintf("update-%d", updateCount)
			if _, err := client.CoreV1().ConfigMaps(ns).Update(setupCtx, cm, metav1.UpdateOptions{}); err == nil {
				updateCount++
			}
		}
	}

	log.Printf("S3: Completed %d high cardinality updates", updateCount)
	return ExpectedMetrics{
		ActionTotal:         updateCount,
		ReloadExecutedTotal: updateCount,
		Description:         fmt.Sprintf("S3: %d updates across %d namespaces", updateCount, len(namespaces)),
	}, nil
}

// NoOpUpdateScenario - Updates that don't actually change data.
type NoOpUpdateScenario struct{}

func (s *NoOpUpdateScenario) Name() string        { return "S4" }
func (s *NoOpUpdateScenario) Description() string { return "No-op updates (same data)" }

func (s *NoOpUpdateScenario) Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error) {
	log.Println("S4: Creating ConfigMaps and Deployments for no-op test...")

	setupCtx := context.Background()

	for i := 0; i < 10; i++ {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("noop-cm-%d", i),
				Namespace: namespace,
			},
			Data: map[string]string{"key": "static-value"},
		}
		client.CoreV1().ConfigMaps(namespace).Create(setupCtx, cm, metav1.CreateOptions{})
		deploy := createDeployment(fmt.Sprintf("noop-deploy-%d", i), namespace, fmt.Sprintf("noop-cm-%d", i))
		client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{})
	}

	if err := waitForDeploymentsReady(setupCtx, client, namespace, 3*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}

	log.Println("S4: Starting no-op updates (annotation changes only)...")

	updateCount := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	endTime := time.Now().Add(duration - 5*time.Second)
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			log.Printf("S4: Completed %d no-op updates", updateCount)
			return ExpectedMetrics{
				ActionTotal:         0,
				ReloadExecutedTotal: 0,
				SkippedTotal:        updateCount,
				Description:         fmt.Sprintf("S4: %d no-op updates, all should be skipped", updateCount),
			}, nil
		case <-ticker.C:
			cmIndex := rand.Intn(10)
			cm, err := client.CoreV1().ConfigMaps(namespace).Get(setupCtx, fmt.Sprintf("noop-cm-%d", cmIndex), metav1.GetOptions{})
			if err != nil {
				continue
			}
			if cm.Annotations == nil {
				cm.Annotations = make(map[string]string)
			}
			cm.Annotations["noop-counter"] = fmt.Sprintf("%d", updateCount)
			if _, err := client.CoreV1().ConfigMaps(namespace).Update(setupCtx, cm, metav1.UpdateOptions{}); err == nil {
				updateCount++
			}
		}
	}

	log.Printf("S4: Completed %d no-op updates (should see 0 actions)", updateCount)
	return ExpectedMetrics{
		ActionTotal:         0,
		ReloadExecutedTotal: 0,
		SkippedTotal:        updateCount,
		Description:         fmt.Sprintf("S4: %d no-op updates, all should be skipped", updateCount),
	}, nil
}

// WorkloadChurnScenario - Deployments created and deleted rapidly.
type WorkloadChurnScenario struct{}

func (s *WorkloadChurnScenario) Name() string        { return "S5" }
func (s *WorkloadChurnScenario) Description() string { return "Workload churn (rapid create/delete)" }

func (s *WorkloadChurnScenario) Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error) {
	log.Println("S5: Creating base ConfigMap...")

	setupCtx := context.Background()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "churn-cm", Namespace: namespace},
		Data:       map[string]string{"key": "value"},
	}
	client.CoreV1().ConfigMaps(namespace).Create(setupCtx, cm, metav1.CreateOptions{})

	log.Println("S5: Starting workload churn...")

	var wg sync.WaitGroup
	var mu sync.Mutex
	deployCounter := 0
	deleteCounter := 0
	cmUpdateCount := 0

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		endTime := time.Now().Add(duration - 5*time.Second)
		for time.Now().Before(endTime) {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				deployName := fmt.Sprintf("churn-deploy-%d", deployCounter)
				deploy := createDeployment(deployName, namespace, "churn-cm")
				if _, err := client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{}); err == nil {
					mu.Lock()
					deployCounter++
					mu.Unlock()
				}
				if deployCounter > 10 {
					oldName := fmt.Sprintf("churn-deploy-%d", deployCounter-10)
					if err := client.AppsV1().Deployments(namespace).Delete(setupCtx, oldName, metav1.DeleteOptions{}); err == nil {
						mu.Lock()
						deleteCounter++
						mu.Unlock()
					}
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		endTime := time.Now().Add(duration - 5*time.Second)
		for time.Now().Before(endTime) {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cm, err := client.CoreV1().ConfigMaps(namespace).Get(setupCtx, "churn-cm", metav1.GetOptions{})
				if err != nil {
					continue
				}
				cm.Data["key"] = fmt.Sprintf("update-%d", cmUpdateCount)
				if _, err := client.CoreV1().ConfigMaps(namespace).Update(setupCtx, cm, metav1.UpdateOptions{}); err == nil {
					mu.Lock()
					cmUpdateCount++
					mu.Unlock()
				}
			}
		}
	}()

	wg.Wait()
	log.Printf("S5: Created %d, deleted %d deployments, %d CM updates", deployCounter, deleteCounter, cmUpdateCount)

	// S5 does NOT set expected values for action_total/reload_executed_total because:
	// - There are ~10 active deployments at any time (creates new, deletes old)
	// - Each CM update triggers reloads on ALL active deployments
	// - Exact counts depend on timing of creates/deletes vs CM updates
	// - "Not found" errors are expected when a deployment is deleted during processing
	// Instead, S5 pass/fail compares old vs new (both should be similar)
	return ExpectedMetrics{
		// No expected values - churn makes exact counts unpredictable
		Description: fmt.Sprintf("S5: Churn test - %d deploys created, %d deleted, %d CM updates, ~10 active deploys at any time", deployCounter, deleteCounter, cmUpdateCount),
	}, nil
}

// ControllerRestartScenario - Restart controller under load.
type ControllerRestartScenario struct {
	ReloaderVersion string
}

func (s *ControllerRestartScenario) Name() string { return "S6" }
func (s *ControllerRestartScenario) Description() string {
	return "Controller restart under load"
}

func (s *ControllerRestartScenario) Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error) {
	log.Println("S6: Creating resources and generating load...")

	setupCtx := context.Background()

	for i := 0; i < 20; i++ {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("restart-cm-%d", i),
				Namespace: namespace,
			},
			Data: map[string]string{"key": "initial"},
		}
		client.CoreV1().ConfigMaps(namespace).Create(setupCtx, cm, metav1.CreateOptions{})
		deploy := createDeployment(fmt.Sprintf("restart-deploy-%d", i), namespace, fmt.Sprintf("restart-cm-%d", i))
		client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{})
	}

	if err := waitForDeploymentsReady(setupCtx, client, namespace, 3*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	updateCount := 0

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		endTime := time.Now().Add(duration - 5*time.Second)
		for time.Now().Before(endTime) {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cmIndex := rand.Intn(20)
				cm, err := client.CoreV1().ConfigMaps(namespace).Get(setupCtx, fmt.Sprintf("restart-cm-%d", cmIndex), metav1.GetOptions{})
				if err != nil {
					continue
				}
				cm.Data["key"] = fmt.Sprintf("update-%d", updateCount)
				if _, err := client.CoreV1().ConfigMaps(namespace).Update(setupCtx, cm, metav1.UpdateOptions{}); err == nil {
					mu.Lock()
					updateCount++
					mu.Unlock()
				}
			}
		}
	}()

	reloaderNS := fmt.Sprintf("reloader-%s", s.ReloaderVersion)
	if s.ReloaderVersion == "" {
		reloaderNS = "reloader-new"
	}

	log.Println("S6: Waiting 20 seconds before restarting controller...")
	time.Sleep(20 * time.Second)

	log.Println("S6: Restarting Reloader pod...")
	pods, err := client.CoreV1().Pods(reloaderNS).List(setupCtx, metav1.ListOptions{
		LabelSelector: "app=reloader",
	})
	if err == nil && len(pods.Items) > 0 {
		client.CoreV1().Pods(reloaderNS).Delete(setupCtx, pods.Items[0].Name, metav1.DeleteOptions{})
	}

	wg.Wait()
	log.Printf("S6: Controller restart scenario completed with %d updates", updateCount)
	return ExpectedMetrics{
		Description: fmt.Sprintf("S6: Restart test - %d updates during restart", updateCount),
	}, nil
}

// APIPressureScenario - Simulate API server pressure with many concurrent requests.
type APIPressureScenario struct{}

func (s *APIPressureScenario) Name() string        { return "S7" }
func (s *APIPressureScenario) Description() string { return "API pressure (many concurrent requests)" }

func (s *APIPressureScenario) Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error) {
	log.Println("S7: Creating resources for API pressure test...")

	const numConfigMaps = 50
	setupCtx := context.Background()

	for i := 0; i < numConfigMaps; i++ {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("api-cm-%d", i),
				Namespace: namespace,
			},
			Data: map[string]string{"key": "value"},
		}
		client.CoreV1().ConfigMaps(namespace).Create(setupCtx, cm, metav1.CreateOptions{})
		deploy := createDeployment(fmt.Sprintf("api-deploy-%d", i), namespace, fmt.Sprintf("api-cm-%d", i))
		client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{})
	}

	if err := waitForDeploymentsReady(setupCtx, client, namespace, 5*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}

	log.Println("S7: Starting concurrent updates from multiple goroutines...")

	updateDuration := duration - 5*time.Second
	if updateDuration < 30*time.Second {
		updateDuration = 30 * time.Second
	}

	updateCtx, updateCancel := context.WithTimeout(context.Background(), updateDuration)
	defer updateCancel()

	endTime := time.Now().Add(updateDuration)
	log.Printf("S7: Will run updates for %v (until %v)", updateDuration, endTime.Format("15:04:05"))

	var wg sync.WaitGroup
	var mu sync.Mutex
	totalUpdates := 0

	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			updateCount := 0
			for time.Now().Before(endTime) {
				select {
				case <-updateCtx.Done():
					return
				case <-ticker.C:
					cmIndex := rand.Intn(numConfigMaps)
					cm, err := client.CoreV1().ConfigMaps(namespace).Get(setupCtx, fmt.Sprintf("api-cm-%d", cmIndex), metav1.GetOptions{})
					if err != nil {
						continue
					}
					cm.Data["key"] = fmt.Sprintf("g%d-update-%d", goroutineID, updateCount)
					if _, err := client.CoreV1().ConfigMaps(namespace).Update(setupCtx, cm, metav1.UpdateOptions{}); err == nil {
						updateCount++
					}
				}
			}
			mu.Lock()
			totalUpdates += updateCount
			mu.Unlock()
			log.Printf("S7: Goroutine %d completed %d updates", goroutineID, updateCount)
		}(g)
	}

	wg.Wait()
	log.Printf("S7: API pressure scenario completed with %d total updates", totalUpdates)
	return ExpectedMetrics{
		ActionTotal:         totalUpdates,
		ReloadExecutedTotal: totalUpdates,
		Description:         fmt.Sprintf("S7: %d concurrent updates from 10 goroutines", totalUpdates),
	}, nil
}

// LargeObjectScenario - Large ConfigMaps/Secrets.
type LargeObjectScenario struct{}

func (s *LargeObjectScenario) Name() string        { return "S8" }
func (s *LargeObjectScenario) Description() string { return "Large ConfigMaps/Secrets (>100KB)" }

func (s *LargeObjectScenario) Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error) {
	log.Println("S8: Creating large ConfigMaps...")

	setupCtx := context.Background()

	largeData := make([]byte, 100*1024)
	for i := range largeData {
		largeData[i] = byte('a' + (i % 26))
	}
	largeValue := string(largeData)

	for i := 0; i < 10; i++ {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("large-cm-%d", i),
				Namespace: namespace,
			},
			Data: map[string]string{
				"large-key-1": largeValue,
				"large-key-2": largeValue,
			},
		}
		if _, err := client.CoreV1().ConfigMaps(namespace).Create(setupCtx, cm, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create large ConfigMap %d: %v", i, err)
		}
		deploy := createDeployment(fmt.Sprintf("large-deploy-%d", i), namespace, fmt.Sprintf("large-cm-%d", i))
		client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{})
	}

	if err := waitForDeploymentsReady(setupCtx, client, namespace, 3*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}

	log.Println("S8: Starting large object updates...")

	updateCount := 0
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	endTime := time.Now().Add(duration - 5*time.Second)
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			log.Printf("S8: Completed %d large object updates", updateCount)
			return ExpectedMetrics{
				ActionTotal:         updateCount,
				ReloadExecutedTotal: updateCount,
				Description:         fmt.Sprintf("S8: %d large object (100KB) updates", updateCount),
			}, nil
		case <-ticker.C:
			cmIndex := rand.Intn(10)
			cm, err := client.CoreV1().ConfigMaps(namespace).Get(setupCtx, fmt.Sprintf("large-cm-%d", cmIndex), metav1.GetOptions{})
			if err != nil {
				continue
			}
			cm.Data["large-key-1"] = largeValue[:len(largeValue)-10] + fmt.Sprintf("-%d", updateCount)
			if _, err := client.CoreV1().ConfigMaps(namespace).Update(setupCtx, cm, metav1.UpdateOptions{}); err != nil {
				log.Printf("Failed to update large ConfigMap: %v", err)
			} else {
				updateCount++
			}
		}
	}

	log.Printf("S8: Completed %d large object updates", updateCount)
	return ExpectedMetrics{
		ActionTotal:         updateCount,
		ReloadExecutedTotal: updateCount,
		Description:         fmt.Sprintf("S8: %d large object (100KB) updates", updateCount),
	}, nil
}

// Helper functions

func waitForDeploymentsReady(ctx context.Context, client kubernetes.Interface, namespace string, timeout time.Duration) error {
	log.Printf("Waiting for all deployments in %s to be ready (timeout: %v)...", namespace, timeout)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		deployments, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list deployments: %w", err)
		}

		allReady := true
		notReady := 0
		for _, d := range deployments.Items {
			if d.Status.ReadyReplicas < *d.Spec.Replicas {
				allReady = false
				notReady++
			}
		}

		if allReady && len(deployments.Items) > 0 {
			log.Printf("All %d deployments in %s are ready", len(deployments.Items), namespace)
			return nil
		}

		log.Printf("Waiting for deployments: %d/%d not ready yet...", notReady, len(deployments.Items))
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for deployments to be ready")
}

func waitForAllNamespacesReady(ctx context.Context, client kubernetes.Interface, namespaces []string, timeout time.Duration) error {
	log.Printf("Waiting for deployments in %d namespaces to be ready...", len(namespaces))

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		allReady := true
		totalDeploys := 0
		notReady := 0

		for _, ns := range namespaces {
			deployments, err := client.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			for _, d := range deployments.Items {
				totalDeploys++
				if d.Status.ReadyReplicas < *d.Spec.Replicas {
					allReady = false
					notReady++
				}
			}
		}

		if allReady && totalDeploys > 0 {
			log.Printf("All %d deployments across %d namespaces are ready", totalDeploys, len(namespaces))
			return nil
		}

		log.Printf("Waiting: %d/%d deployments not ready yet...", notReady, totalDeploys)
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for deployments to be ready")
}

func createDeployment(name, namespace, configMapName string) *appsv1.Deployment {
	replicas := int32(1)
	maxSurge := intstr.FromInt(1)
	maxUnavailable := intstr.FromInt(1)
	terminationGracePeriod := int64(0)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"reloader.stakater.com/auto": "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Containers: []corev1.Container{
						{
							Name:    "app",
							Image:   "gcr.io/google-containers/busybox:1.27",
							Command: []string{"sh", "-c", "sleep 999999999"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1m"),
									corev1.ResourceMemory: resource.MustParse("4Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("16Mi"),
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMapName,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func createDeploymentWithSecret(name, namespace, secretName string) *appsv1.Deployment {
	replicas := int32(1)
	maxSurge := intstr.FromInt(1)
	maxUnavailable := intstr.FromInt(1)
	terminationGracePeriod := int64(0)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"reloader.stakater.com/auto": "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Containers: []corev1.Container{
						{
							Name:    "app",
							Image:   "gcr.io/google-containers/busybox:1.27",
							Command: []string{"sh", "-c", "sleep 999999999"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1m"),
									corev1.ResourceMemory: resource.MustParse("4Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("16Mi"),
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func createDeploymentWithBoth(name, namespace, configMapName, secretName string) *appsv1.Deployment {
	replicas := int32(1)
	maxSurge := intstr.FromInt(1)
	maxUnavailable := intstr.FromInt(1)
	terminationGracePeriod := int64(0)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"reloader.stakater.com/auto": "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Containers: []corev1.Container{
						{
							Name:    "app",
							Image:   "gcr.io/google-containers/busybox:1.27",
							Command: []string{"sh", "-c", "sleep 999999999"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1m"),
									corev1.ResourceMemory: resource.MustParse("4Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("16Mi"),
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMapName,
										},
									},
								},
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// SecretsAndMixedScenario - Tests Secrets and mixed ConfigMap+Secret workloads.
type SecretsAndMixedScenario struct{}

func (s *SecretsAndMixedScenario) Name() string { return "S10" }
func (s *SecretsAndMixedScenario) Description() string {
	return "Secrets and mixed ConfigMap+Secret workloads"
}

func (s *SecretsAndMixedScenario) Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error) {
	log.Println("S10: Creating Secrets, ConfigMaps, and mixed workloads...")

	const numSecrets = 5
	const numConfigMaps = 5
	const numSecretOnlyDeploys = 5
	const numConfigMapOnlyDeploys = 3
	const numMixedDeploys = 2

	setupCtx := context.Background()

	// Create Secrets
	for i := 0; i < numSecrets; i++ {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("mixed-secret-%d", i),
				Namespace: namespace,
			},
			StringData: map[string]string{
				"password": fmt.Sprintf("initial-secret-%d", i),
			},
		}
		if _, err := client.CoreV1().Secrets(namespace).Create(setupCtx, secret, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create Secret %s: %v", secret.Name, err)
		}
	}

	// Create ConfigMaps
	for i := 0; i < numConfigMaps; i++ {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("mixed-cm-%d", i),
				Namespace: namespace,
			},
			Data: map[string]string{
				"config": fmt.Sprintf("initial-config-%d", i),
			},
		}
		if _, err := client.CoreV1().ConfigMaps(namespace).Create(setupCtx, cm, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create ConfigMap %s: %v", cm.Name, err)
		}
	}

	// Create Secret-only deployments
	for i := 0; i < numSecretOnlyDeploys; i++ {
		deploy := createDeploymentWithSecret(
			fmt.Sprintf("secret-only-deploy-%d", i),
			namespace,
			fmt.Sprintf("mixed-secret-%d", i%numSecrets),
		)
		if _, err := client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create Secret-only Deployment: %v", err)
		}
	}

	// Create ConfigMap-only deployments
	for i := 0; i < numConfigMapOnlyDeploys; i++ {
		deploy := createDeployment(
			fmt.Sprintf("cm-only-deploy-%d", i),
			namespace,
			fmt.Sprintf("mixed-cm-%d", i%numConfigMaps),
		)
		if _, err := client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create ConfigMap-only Deployment: %v", err)
		}
	}

	// Create mixed deployments (using both Secret and ConfigMap)
	for i := 0; i < numMixedDeploys; i++ {
		deploy := createDeploymentWithBoth(
			fmt.Sprintf("mixed-deploy-%d", i),
			namespace,
			fmt.Sprintf("mixed-cm-%d", i%numConfigMaps),
			fmt.Sprintf("mixed-secret-%d", i%numSecrets),
		)
		if _, err := client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create mixed Deployment: %v", err)
		}
	}

	if err := waitForDeploymentsReady(setupCtx, client, namespace, 3*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}

	log.Println("S10: Starting alternating Secret and ConfigMap updates...")

	secretUpdateCount := 0
	cmUpdateCount := 0
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	updateSecret := true // Alternate between Secret and ConfigMap updates

	endTime := time.Now().Add(duration - 5*time.Second)
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return s.calculateExpected(secretUpdateCount, cmUpdateCount, numSecretOnlyDeploys, numConfigMapOnlyDeploys, numMixedDeploys), nil
		case <-ticker.C:
			if updateSecret {
				// Update a random Secret
				secretIndex := rand.Intn(numSecrets)
				secret, err := client.CoreV1().Secrets(namespace).Get(setupCtx, fmt.Sprintf("mixed-secret-%d", secretIndex), metav1.GetOptions{})
				if err != nil {
					continue
				}
				secret.StringData = map[string]string{
					"password": fmt.Sprintf("updated-secret-%d-%d", secretIndex, secretUpdateCount),
				}
				if _, err := client.CoreV1().Secrets(namespace).Update(setupCtx, secret, metav1.UpdateOptions{}); err == nil {
					secretUpdateCount++
				}
			} else {
				// Update a random ConfigMap
				cmIndex := rand.Intn(numConfigMaps)
				cm, err := client.CoreV1().ConfigMaps(namespace).Get(setupCtx, fmt.Sprintf("mixed-cm-%d", cmIndex), metav1.GetOptions{})
				if err != nil {
					continue
				}
				cm.Data["config"] = fmt.Sprintf("updated-config-%d-%d", cmIndex, cmUpdateCount)
				if _, err := client.CoreV1().ConfigMaps(namespace).Update(setupCtx, cm, metav1.UpdateOptions{}); err == nil {
					cmUpdateCount++
				}
			}
			updateSecret = !updateSecret
		}
	}

	log.Printf("S10: Completed %d Secret updates and %d ConfigMap updates", secretUpdateCount, cmUpdateCount)
	return s.calculateExpected(secretUpdateCount, cmUpdateCount, numSecretOnlyDeploys, numConfigMapOnlyDeploys, numMixedDeploys), nil
}

func (s *SecretsAndMixedScenario) calculateExpected(secretUpdates, cmUpdates, secretOnlyDeploys, cmOnlyDeploys, mixedDeploys int) ExpectedMetrics {
	// Secret updates trigger: secret-only deploys + mixed deploys
	secretTriggeredReloads := secretUpdates * (secretOnlyDeploys + mixedDeploys)
	// ConfigMap updates trigger: cm-only deploys + mixed deploys
	cmTriggeredReloads := cmUpdates * (cmOnlyDeploys + mixedDeploys)
	totalExpectedReloads := secretTriggeredReloads + cmTriggeredReloads

	return ExpectedMetrics{
		ActionTotal:         totalExpectedReloads,
		ReloadExecutedTotal: totalExpectedReloads,
		Description: fmt.Sprintf("S10: %d Secret updates (→%d reloads) + %d CM updates (→%d reloads) = %d total",
			secretUpdates, secretTriggeredReloads, cmUpdates, cmTriggeredReloads, totalExpectedReloads),
	}
}

// MultiWorkloadTypeScenario - Tests all supported workload types with a shared ConfigMap.
type MultiWorkloadTypeScenario struct{}

func (s *MultiWorkloadTypeScenario) Name() string { return "S9" }
func (s *MultiWorkloadTypeScenario) Description() string {
	return "Multi-workload types (Deploy, StatefulSet, DaemonSet, Job, CronJob)"
}

func (s *MultiWorkloadTypeScenario) Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error) {
	log.Println("S9: Creating shared ConfigMap and multiple workload types...")

	const numDeployments = 5
	const numStatefulSets = 3
	const numDaemonSets = 2

	setupCtx := context.Background()

	// Create shared ConfigMap
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multi-type-cm",
			Namespace: namespace,
		},
		Data: map[string]string{"config": "initial"},
	}
	if _, err := client.CoreV1().ConfigMaps(namespace).Create(setupCtx, cm, metav1.CreateOptions{}); err != nil {
		return ExpectedMetrics{}, fmt.Errorf("failed to create shared ConfigMap: %w", err)
	}

	// Create Deployments
	for i := 0; i < numDeployments; i++ {
		deploy := createDeployment(fmt.Sprintf("multi-deploy-%d", i), namespace, "multi-type-cm")
		if _, err := client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create Deployment %d: %v", i, err)
		}
	}

	// Create StatefulSets
	for i := 0; i < numStatefulSets; i++ {
		sts := createStatefulSet(fmt.Sprintf("multi-sts-%d", i), namespace, "multi-type-cm")
		if _, err := client.AppsV1().StatefulSets(namespace).Create(setupCtx, sts, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create StatefulSet %d: %v", i, err)
		}
	}

	// Create DaemonSets
	for i := 0; i < numDaemonSets; i++ {
		ds := createDaemonSet(fmt.Sprintf("multi-ds-%d", i), namespace, "multi-type-cm")
		if _, err := client.AppsV1().DaemonSets(namespace).Create(setupCtx, ds, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create DaemonSet %d: %v", i, err)
		}
	}

	// Wait for workloads to be ready
	if err := waitForDeploymentsReady(setupCtx, client, namespace, 3*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}
	if err := waitForStatefulSetsReady(setupCtx, client, namespace, 3*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}
	if err := waitForDaemonSetsReady(setupCtx, client, namespace, 3*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}

	log.Println("S9: Starting ConfigMap updates to trigger reloads on all workload types...")

	updateCount := 0
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	endTime := time.Now().Add(duration - 5*time.Second)
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return s.calculateExpected(updateCount, numDeployments, numStatefulSets, numDaemonSets), nil
		case <-ticker.C:
			cm, err := client.CoreV1().ConfigMaps(namespace).Get(setupCtx, "multi-type-cm", metav1.GetOptions{})
			if err != nil {
				continue
			}
			cm.Data["config"] = fmt.Sprintf("update-%d", updateCount)
			if _, err := client.CoreV1().ConfigMaps(namespace).Update(setupCtx, cm, metav1.UpdateOptions{}); err != nil {
				log.Printf("Failed to update shared ConfigMap: %v", err)
			} else {
				updateCount++
				log.Printf("S9: Updated shared ConfigMap (update #%d)", updateCount)
			}
		}
	}

	log.Printf("S9: Completed %d ConfigMap updates", updateCount)
	return s.calculateExpected(updateCount, numDeployments, numStatefulSets, numDaemonSets), nil
}

func (s *MultiWorkloadTypeScenario) calculateExpected(updateCount, numDeployments, numStatefulSets, numDaemonSets int) ExpectedMetrics {
	// Each CM update triggers reload on all workloads
	totalWorkloads := numDeployments + numStatefulSets + numDaemonSets
	expectedReloads := updateCount * totalWorkloads

	return ExpectedMetrics{
		ActionTotal:           expectedReloads,
		ReloadExecutedTotal:   expectedReloads,
		WorkloadsMatchedTotal: expectedReloads,
		Description: fmt.Sprintf("S9: %d CM updates × %d workloads (%d Deploys + %d STS + %d DS) = %d reloads",
			updateCount, totalWorkloads, numDeployments, numStatefulSets, numDaemonSets, expectedReloads),
	}
}

func createStatefulSet(name, namespace, configMapName string) *appsv1.StatefulSet {
	replicas := int32(1)
	terminationGracePeriod := int64(0)

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"reloader.stakater.com/auto": "true",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: name,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Containers: []corev1.Container{
						{
							Name:    "app",
							Image:   "gcr.io/google-containers/busybox:1.27",
							Command: []string{"sh", "-c", "sleep 999999999"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1m"),
									corev1.ResourceMemory: resource.MustParse("4Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("16Mi"),
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMapName,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func createDaemonSet(name, namespace, configMapName string) *appsv1.DaemonSet {
	terminationGracePeriod := int64(0)

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"reloader.stakater.com/auto": "true",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					// Use tolerations to run on all nodes including control-plane
					Tolerations: []corev1.Toleration{
						{
							Key:      "node-role.kubernetes.io/control-plane",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "node-role.kubernetes.io/master",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "app",
							Image:   "gcr.io/google-containers/busybox:1.27",
							Command: []string{"sh", "-c", "sleep 999999999"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1m"),
									corev1.ResourceMemory: resource.MustParse("4Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("16Mi"),
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMapName,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func waitForStatefulSetsReady(ctx context.Context, client kubernetes.Interface, namespace string, timeout time.Duration) error {
	log.Printf("Waiting for all StatefulSets in %s to be ready (timeout: %v)...", namespace, timeout)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		stsList, err := client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list StatefulSets: %w", err)
		}

		if len(stsList.Items) == 0 {
			log.Printf("No StatefulSets found in %s", namespace)
			return nil
		}

		allReady := true
		notReady := 0
		for _, sts := range stsList.Items {
			if sts.Status.ReadyReplicas < *sts.Spec.Replicas {
				allReady = false
				notReady++
			}
		}

		if allReady {
			log.Printf("All %d StatefulSets in %s are ready", len(stsList.Items), namespace)
			return nil
		}

		log.Printf("Waiting for StatefulSets: %d/%d not ready yet...", notReady, len(stsList.Items))
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for StatefulSets to be ready")
}

func waitForDaemonSetsReady(ctx context.Context, client kubernetes.Interface, namespace string, timeout time.Duration) error {
	log.Printf("Waiting for all DaemonSets in %s to be ready (timeout: %v)...", namespace, timeout)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		dsList, err := client.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list DaemonSets: %w", err)
		}

		if len(dsList.Items) == 0 {
			log.Printf("No DaemonSets found in %s", namespace)
			return nil
		}

		allReady := true
		notReady := 0
		for _, ds := range dsList.Items {
			if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
				allReady = false
				notReady++
			}
		}

		if allReady {
			log.Printf("All %d DaemonSets in %s are ready", len(dsList.Items), namespace)
			return nil
		}

		log.Printf("Waiting for DaemonSets: %d/%d not ready yet...", notReady, len(dsList.Items))
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for DaemonSets to be ready")
}

// ComplexReferencesScenario - Tests init containers, valueFrom, and projected volumes.
type ComplexReferencesScenario struct{}

func (s *ComplexReferencesScenario) Name() string { return "S13" }
func (s *ComplexReferencesScenario) Description() string {
	return "Complex references (init containers, valueFrom, projected volumes)"
}

func (s *ComplexReferencesScenario) Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error) {
	log.Println("S13: Creating ConfigMaps and complex deployments with various reference types...")

	const numConfigMaps = 5
	const numDeployments = 5

	setupCtx := context.Background()

	// Create ConfigMaps with multiple keys
	for i := 0; i < numConfigMaps; i++ {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("complex-cm-%d", i),
				Namespace: namespace,
			},
			Data: map[string]string{
				"key1":   fmt.Sprintf("value1-%d", i),
				"key2":   fmt.Sprintf("value2-%d", i),
				"config": fmt.Sprintf("config-%d", i),
			},
		}
		if _, err := client.CoreV1().ConfigMaps(namespace).Create(setupCtx, cm, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create ConfigMap %s: %v", cm.Name, err)
		}
	}

	// Create complex deployments with various reference types
	for i := 0; i < numDeployments; i++ {
		// Each deployment references multiple ConfigMaps in different ways
		primaryCM := fmt.Sprintf("complex-cm-%d", i)
		secondaryCM := fmt.Sprintf("complex-cm-%d", (i+1)%numConfigMaps)

		deploy := createComplexDeployment(
			fmt.Sprintf("complex-deploy-%d", i),
			namespace,
			primaryCM,
			secondaryCM,
		)
		if _, err := client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create complex Deployment: %v", err)
		}
	}

	if err := waitForDeploymentsReady(setupCtx, client, namespace, 3*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}

	log.Println("S13: Starting ConfigMap updates to test all reference types...")

	updateCount := 0
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	endTime := time.Now().Add(duration - 5*time.Second)
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return s.calculateExpected(updateCount, numConfigMaps, numDeployments), nil
		case <-ticker.C:
			// Update a random ConfigMap
			cmIndex := rand.Intn(numConfigMaps)
			cm, err := client.CoreV1().ConfigMaps(namespace).Get(setupCtx, fmt.Sprintf("complex-cm-%d", cmIndex), metav1.GetOptions{})
			if err != nil {
				continue
			}
			cm.Data["key1"] = fmt.Sprintf("updated-value1-%d-%d", cmIndex, updateCount)
			cm.Data["config"] = fmt.Sprintf("updated-config-%d-%d", cmIndex, updateCount)
			if _, err := client.CoreV1().ConfigMaps(namespace).Update(setupCtx, cm, metav1.UpdateOptions{}); err != nil {
				log.Printf("Failed to update ConfigMap: %v", err)
			} else {
				updateCount++
				log.Printf("S13: Updated complex-cm-%d (update #%d)", cmIndex, updateCount)
			}
		}
	}

	log.Printf("S13: Completed %d ConfigMap updates", updateCount)
	return s.calculateExpected(updateCount, numConfigMaps, numDeployments), nil
}

func (s *ComplexReferencesScenario) calculateExpected(updateCount, numConfigMaps, numDeployments int) ExpectedMetrics {
	// Each ConfigMap is referenced by:
	// - 1 deployment as primary (envFrom in init + valueFrom in main + volume mount)
	// - 1 deployment as secondary (projected volume)
	// So each CM update triggers 2 deployments (on average with random updates)
	// But since we're randomly updating, each update affects those 2 deployments
	expectedReloadsPerUpdate := 2
	expectedReloads := updateCount * expectedReloadsPerUpdate

	return ExpectedMetrics{
		ActionTotal:         expectedReloads,
		ReloadExecutedTotal: expectedReloads,
		Description: fmt.Sprintf("S13: %d CM updates × ~%d affected deploys = ~%d reloads (init containers, valueFrom, volumes, projected)",
			updateCount, expectedReloadsPerUpdate, expectedReloads),
	}
}

// PauseResumeScenario - Tests pause-period functionality under rapid updates.
type PauseResumeScenario struct{}

func (s *PauseResumeScenario) Name() string { return "S12" }
func (s *PauseResumeScenario) Description() string {
	return "Pause & Resume (rapid updates with pause-period)"
}

func (s *PauseResumeScenario) Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error) {
	log.Println("S12: Creating ConfigMaps and Deployments with pause-period annotation...")

	const numConfigMaps = 10
	const numDeployments = 10
	const pausePeriod = 15 * time.Second
	const updateInterval = 2 * time.Second

	setupCtx := context.Background()

	// Create ConfigMaps
	for i := 0; i < numConfigMaps; i++ {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("pause-cm-%d", i),
				Namespace: namespace,
			},
			Data: map[string]string{"key": "initial-value"},
		}
		if _, err := client.CoreV1().ConfigMaps(namespace).Create(setupCtx, cm, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create ConfigMap %s: %v", cm.Name, err)
		}
	}

	// Create Deployments with pause-period annotation
	for i := 0; i < numDeployments; i++ {
		deploy := createDeploymentWithPause(
			fmt.Sprintf("pause-deploy-%d", i),
			namespace,
			fmt.Sprintf("pause-cm-%d", i),
			pausePeriod,
		)
		if _, err := client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create Deployment: %v", err)
		}
	}

	if err := waitForDeploymentsReady(setupCtx, client, namespace, 3*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}

	log.Printf("S12: Starting rapid ConfigMap updates (every %v) with %v pause-period...", updateInterval, pausePeriod)

	updateCount := 0
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()

	endTime := time.Now().Add(duration - 5*time.Second)
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return s.calculateExpected(updateCount, duration, updateInterval, pausePeriod), nil
		case <-ticker.C:
			// Update a random ConfigMap
			cmIndex := rand.Intn(numConfigMaps)
			cm, err := client.CoreV1().ConfigMaps(namespace).Get(setupCtx, fmt.Sprintf("pause-cm-%d", cmIndex), metav1.GetOptions{})
			if err != nil {
				continue
			}
			cm.Data["key"] = fmt.Sprintf("update-%d-%d", cmIndex, updateCount)
			if _, err := client.CoreV1().ConfigMaps(namespace).Update(setupCtx, cm, metav1.UpdateOptions{}); err != nil {
				log.Printf("Failed to update ConfigMap: %v", err)
			} else {
				updateCount++
			}
		}
	}

	log.Printf("S12: Completed %d rapid updates (pause-period should reduce actual reloads)", updateCount)
	return s.calculateExpected(updateCount, duration, updateInterval, pausePeriod), nil
}

func (s *PauseResumeScenario) calculateExpected(updateCount int, duration, updateInterval, pausePeriod time.Duration) ExpectedMetrics {
	// With pause-period, we expect fewer reloads than updates
	// Each deployment gets updates at random, and pause-period prevents rapid consecutive reloads
	// The exact count depends on the distribution of updates across ConfigMaps
	// Rough estimate: each CM gets updated ~(updateCount/10) times
	// With 15s pause and 2s interval, we get roughly 1 reload per pause period per CM
	// So expected reloads ≈ duration / pausePeriod per deployment = (duration/pausePeriod) * numDeployments

	// This is an approximation - the actual value depends on random distribution
	expectedCycles := int(duration / pausePeriod)
	if expectedCycles < 1 {
		expectedCycles = 1
	}

	return ExpectedMetrics{
		// Don't set exact expected values since pause-period makes counts unpredictable
		// The scenario validates that reloads << updates due to pause behavior
		Description: fmt.Sprintf("S12: %d updates with %v pause-period (expect ~%d reload cycles, actual reloads << updates)",
			updateCount, pausePeriod, expectedCycles),
	}
}

// AnnotationStrategyScenario - Tests annotation-based reload strategy.
// This scenario deploys its own Reloader instance with --reload-strategy=annotations.
type AnnotationStrategyScenario struct {
	// Image is the Reloader image to use. Must be set before running.
	Image string
}

func (s *AnnotationStrategyScenario) Name() string { return "S11" }
func (s *AnnotationStrategyScenario) Description() string {
	return "Annotation reload strategy (--reload-strategy=annotations)"
}

func (s *AnnotationStrategyScenario) Run(ctx context.Context, client kubernetes.Interface, namespace string, duration time.Duration) (ExpectedMetrics, error) {
	if s.Image == "" {
		return ExpectedMetrics{}, fmt.Errorf("S11 requires Image to be set (use the same image as --new-image)")
	}

	log.Println("S11: Deploying Reloader with --reload-strategy=annotations...")

	// Deploy S11's own Reloader instance
	reloaderNS := "reloader-s11"
	mgr := reloader.NewManager(reloader.Config{
		Version:        "s11",
		Image:          s.Image,
		Namespace:      reloaderNS,
		ReloadStrategy: "annotations",
	})

	if err := mgr.Deploy(ctx); err != nil {
		return ExpectedMetrics{}, fmt.Errorf("deploying S11 reloader: %w", err)
	}

	// Ensure cleanup on exit
	defer func() {
		log.Println("S11: Cleaning up S11-specific Reloader...")
		cleanupCtx := context.Background()
		if err := mgr.Cleanup(cleanupCtx); err != nil {
			log.Printf("Warning: failed to cleanup S11 reloader: %v", err)
		}
	}()

	log.Println("S11: Creating ConfigMaps and Deployments...")

	const numConfigMaps = 10
	const numDeployments = 10

	setupCtx := context.Background()

	// Create ConfigMaps
	for i := 0; i < numConfigMaps; i++ {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("annot-cm-%d", i),
				Namespace: namespace,
			},
			Data: map[string]string{"key": "initial-value"},
		}
		if _, err := client.CoreV1().ConfigMaps(namespace).Create(setupCtx, cm, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create ConfigMap %s: %v", cm.Name, err)
		}
	}

	// Create Deployments
	for i := 0; i < numDeployments; i++ {
		deploy := createDeployment(fmt.Sprintf("annot-deploy-%d", i), namespace, fmt.Sprintf("annot-cm-%d", i))
		if _, err := client.AppsV1().Deployments(namespace).Create(setupCtx, deploy, metav1.CreateOptions{}); err != nil {
			log.Printf("Failed to create Deployment: %v", err)
		}
	}

	if err := waitForDeploymentsReady(setupCtx, client, namespace, 3*time.Minute); err != nil {
		log.Printf("Warning: %v - continuing anyway", err)
	}

	log.Println("S11: Starting ConfigMap updates with annotation strategy...")

	updateCount := 0
	annotationUpdatesSeen := 0
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	endTime := time.Now().Add(duration - 10*time.Second) // Extra time for cleanup
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return s.calculateExpected(updateCount, annotationUpdatesSeen), nil
		case <-ticker.C:
			// Update a random ConfigMap
			cmIndex := rand.Intn(numConfigMaps)
			cm, err := client.CoreV1().ConfigMaps(namespace).Get(setupCtx, fmt.Sprintf("annot-cm-%d", cmIndex), metav1.GetOptions{})
			if err != nil {
				continue
			}
			cm.Data["key"] = fmt.Sprintf("update-%d-%d", cmIndex, updateCount)
			if _, err := client.CoreV1().ConfigMaps(namespace).Update(setupCtx, cm, metav1.UpdateOptions{}); err != nil {
				log.Printf("Failed to update ConfigMap: %v", err)
			} else {
				updateCount++
			}

			// Periodically check for annotation updates on deployments
			if updateCount%10 == 0 {
				deploy, err := client.AppsV1().Deployments(namespace).Get(setupCtx, fmt.Sprintf("annot-deploy-%d", cmIndex), metav1.GetOptions{})
				if err == nil {
					if _, hasAnnotation := deploy.Spec.Template.Annotations["reloader.stakater.com/last-reloaded-from"]; hasAnnotation {
						annotationUpdatesSeen++
					}
				}
			}
		}
	}

	// Final check: verify annotation strategy is working
	log.Println("S11: Verifying annotation-based reload...")
	time.Sleep(5 * time.Second) // Allow time for final updates to propagate

	deploysWithAnnotation := 0
	for i := 0; i < numDeployments; i++ {
		deploy, err := client.AppsV1().Deployments(namespace).Get(setupCtx, fmt.Sprintf("annot-deploy-%d", i), metav1.GetOptions{})
		if err != nil {
			continue
		}
		if deploy.Spec.Template.Annotations != nil {
			if _, ok := deploy.Spec.Template.Annotations["reloader.stakater.com/last-reloaded-from"]; ok {
				deploysWithAnnotation++
			}
		}
	}

	log.Printf("S11: Completed %d updates, %d deployments have reload annotation", updateCount, deploysWithAnnotation)
	return s.calculateExpected(updateCount, deploysWithAnnotation), nil
}

func (s *AnnotationStrategyScenario) calculateExpected(updateCount, deploysWithAnnotation int) ExpectedMetrics {
	return ExpectedMetrics{
		ActionTotal:         updateCount,
		ReloadExecutedTotal: updateCount,
		Description: fmt.Sprintf("S11: %d updates with annotation strategy, %d deployments received annotation",
			updateCount, deploysWithAnnotation),
	}
}

func createDeploymentWithPause(name, namespace, configMapName string, pausePeriod time.Duration) *appsv1.Deployment {
	replicas := int32(1)
	maxSurge := intstr.FromInt(1)
	maxUnavailable := intstr.FromInt(1)
	terminationGracePeriod := int64(0)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"reloader.stakater.com/auto": "true",
				// Deployment-specific pause-period annotation
				"deployment.reloader.stakater.com/pause-period": fmt.Sprintf("%ds", int(pausePeriod.Seconds())),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					Containers: []corev1.Container{
						{
							Name:    "app",
							Image:   "gcr.io/google-containers/busybox:1.27",
							Command: []string{"sh", "-c", "sleep 999999999"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1m"),
									corev1.ResourceMemory: resource.MustParse("4Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("16Mi"),
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMapName,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// createComplexDeployment creates a deployment with multiple ConfigMap reference types.
// - Init container using envFrom
// - Main container using env.valueFrom.configMapKeyRef
// - Sidecar container using volume mount
// - Projected volume combining multiple ConfigMaps
func createComplexDeployment(name, namespace, primaryCM, secondaryCM string) *appsv1.Deployment {
	replicas := int32(1)
	maxSurge := intstr.FromInt(1)
	maxUnavailable := intstr.FromInt(1)
	terminationGracePeriod := int64(0)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"reloader.stakater.com/auto": "true",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					// Init container using envFrom
					InitContainers: []corev1.Container{
						{
							Name:    "init",
							Image:   "gcr.io/google-containers/busybox:1.27",
							Command: []string{"sh", "-c", "echo Init done"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1m"),
									corev1.ResourceMemory: resource.MustParse("4Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("16Mi"),
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: primaryCM,
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						// Main container using valueFrom (individual keys)
						{
							Name:    "main",
							Image:   "gcr.io/google-containers/busybox:1.27",
							Command: []string{"sh", "-c", "sleep 999999999"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1m"),
									corev1.ResourceMemory: resource.MustParse("4Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("16Mi"),
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "CONFIG_KEY1",
									ValueFrom: &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: primaryCM,
											},
											Key: "key1",
										},
									},
								},
								{
									Name: "CONFIG_KEY2",
									ValueFrom: &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: primaryCM,
											},
											Key: "key2",
										},
									},
								},
							},
						},
						// Sidecar using volume mount
						{
							Name:    "sidecar",
							Image:   "gcr.io/google-containers/busybox:1.27",
							Command: []string{"sh", "-c", "sleep 999999999"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1m"),
									corev1.ResourceMemory: resource.MustParse("4Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("16Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-volume",
									MountPath: "/etc/config",
								},
								{
									Name:      "projected-volume",
									MountPath: "/etc/projected",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						// Regular ConfigMap volume
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: primaryCM,
									},
								},
							},
						},
						// Projected volume combining multiple ConfigMaps
						{
							Name: "projected-volume",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									Sources: []corev1.VolumeProjection{
										{
											ConfigMap: &corev1.ConfigMapProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: primaryCM,
												},
												Items: []corev1.KeyToPath{
													{
														Key:  "key1",
														Path: "primary-key1",
													},
												},
											},
										},
										{
											ConfigMap: &corev1.ConfigMapProjection{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: secondaryCM,
												},
												Items: []corev1.KeyToPath{
													{
														Key:  "key1",
														Path: "secondary-key1",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
