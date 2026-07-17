package flags

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

var _ = Describe("Watch Namespaces (scoped mode) Flag Tests", Serial, func() {
	var (
		deploymentName string
		configMapName  string
		watchedNS      string
		unwatchedNS    string
		adapter        *utils.DeploymentAdapter
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		watchedNS = "watched-" + utils.RandName("ns")
		unwatchedNS = "unwatched-" + utils.RandName("ns")
		adapter = utils.NewDeploymentAdapter(kubeClient)

		// The watched namespace must exist before install: in scoped mode the
		// chart creates a Role/RoleBinding in it.
		Expect(utils.CreateNamespace(ctx, kubeClient, watchedNS)).To(Succeed())
		Expect(utils.CreateNamespace(ctx, kubeClient, unwatchedNS)).To(Succeed())

		err := deployReloaderWithFlags(map[string]string{
			"reloader.watchGlobally": "false",
			"reloader.namespaces":    fmt.Sprintf("{%s}", watchedNS),
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(waitForReloaderReady()).To(Succeed())
	})

	AfterEach(func() {
		_ = utils.DeleteDeployment(ctx, kubeClient, watchedNS, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, watchedNS, configMapName)
		_ = utils.DeleteDeployment(ctx, kubeClient, unwatchedNS, deploymentName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, unwatchedNS, configMapName)
		_ = undeployReloader()
		_ = utils.DeleteNamespace(ctx, kubeClient, watchedNS)
		_ = utils.DeleteNamespace(ctx, kubeClient, unwatchedNS)
	})

	It("should reload workloads in a watched namespace", func() {
		By("Creating a ConfigMap in the watched namespace")
		_, err := utils.CreateConfigMap(ctx, kubeClient, watchedNS, configMapName,
			map[string]string{"key": "initial"}, nil)
		Expect(err).NotTo(HaveOccurred())

		By("Creating a Deployment in the watched namespace with auto annotation")
		_, err = utils.CreateDeployment(ctx, kubeClient, watchedNS, deploymentName,
			utils.WithConfigMapEnvFrom(configMapName),
			utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
		)
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for Deployment to be ready")
		Expect(adapter.WaitReady(ctx, watchedNS, deploymentName, utils.WorkloadReadyTimeout)).To(Succeed())

		By("Updating the ConfigMap")
		// Capture the reload-annotation baseline before the trigger to avoid the
		// TOCTOU race where Reloader reloads before WaitReloaded records its baseline.
		priorReload, err := adapter.GetPodTemplateAnnotation(ctx, watchedNS, deploymentName, utils.AnnotationLastReloadedFrom)
		Expect(err).NotTo(HaveOccurred())
		err = utils.UpdateConfigMap(ctx, kubeClient, watchedNS, configMapName, map[string]string{"key": "updated"})
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for Deployment to be reloaded (watched namespace should work)")
		reloaded, err := adapter.WaitReloadedFrom(ctx, watchedNS, deploymentName,
			utils.AnnotationLastReloadedFrom, priorReload, utils.ReloadTimeout)
		Expect(err).NotTo(HaveOccurred())
		Expect(reloaded).To(BeTrue(), "Deployment in a watched namespace should reload")
	})

	It("should NOT reload workloads in an unwatched namespace", func() {
		By("Creating a ConfigMap in an unwatched namespace")
		_, err := utils.CreateConfigMap(ctx, kubeClient, unwatchedNS, configMapName,
			map[string]string{"key": "initial"}, nil)
		Expect(err).NotTo(HaveOccurred())

		By("Creating a Deployment in an unwatched namespace with auto annotation")
		_, err = utils.CreateDeployment(ctx, kubeClient, unwatchedNS, deploymentName,
			utils.WithConfigMapEnvFrom(configMapName),
			utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
		)
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for Deployment to be ready")
		Expect(adapter.WaitReady(ctx, unwatchedNS, deploymentName, utils.WorkloadReadyTimeout)).To(Succeed())

		By("Updating the ConfigMap in the unwatched namespace")
		// Capture the reload-annotation baseline before the trigger to avoid the
		// TOCTOU race where Reloader reloads before WaitReloaded records its baseline.
		priorReload, err := adapter.GetPodTemplateAnnotation(ctx, unwatchedNS, deploymentName, utils.AnnotationLastReloadedFrom)
		Expect(err).NotTo(HaveOccurred())
		err = utils.UpdateConfigMap(ctx, kubeClient, unwatchedNS, configMapName, map[string]string{"key": "updated"})
		Expect(err).NotTo(HaveOccurred())

		By("Verifying Deployment was NOT reloaded (namespace not in --namespaces)")
		time.Sleep(utils.NegativeTestWait)
		reloaded, err := adapter.WaitReloadedFrom(ctx, unwatchedNS, deploymentName,
			utils.AnnotationLastReloadedFrom, priorReload, utils.ShortTimeout)
		Expect(err).NotTo(HaveOccurred())
		Expect(reloaded).To(BeFalse(), "Deployment in an unwatched namespace should NOT reload")
	})
})

var _ = Describe("Watch Multiple Namespaces (scoped mode) Flag Tests", Serial, func() {
	var (
		deploymentName string
		configMapName  string
		watchedA       string
		watchedB       string
		unwatchedNS    string
		adapter        *utils.DeploymentAdapter
	)

	BeforeEach(func() {
		deploymentName = utils.RandName("deploy")
		configMapName = utils.RandName("cm")
		watchedA = "watched-a-" + utils.RandName("ns")
		watchedB = "watched-b-" + utils.RandName("ns")
		unwatchedNS = "unwatched-" + utils.RandName("ns")
		adapter = utils.NewDeploymentAdapter(kubeClient)

		// Both watched namespaces must exist before install: in scoped mode the
		// chart creates a Role/RoleBinding in each.
		Expect(utils.CreateNamespace(ctx, kubeClient, watchedA)).To(Succeed())
		Expect(utils.CreateNamespace(ctx, kubeClient, watchedB)).To(Succeed())
		Expect(utils.CreateNamespace(ctx, kubeClient, unwatchedNS)).To(Succeed())

		err := deployReloaderWithFlags(map[string]string{
			"reloader.watchGlobally": "false",
			"reloader.namespaces":    fmt.Sprintf("{%s,%s}", watchedA, watchedB),
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(waitForReloaderReady()).To(Succeed())
	})

	AfterEach(func() {
		for _, ns := range []string{watchedA, watchedB, unwatchedNS} {
			_ = utils.DeleteDeployment(ctx, kubeClient, ns, deploymentName)
			_ = utils.DeleteConfigMap(ctx, kubeClient, ns, configMapName)
		}
		_ = undeployReloader()
		for _, ns := range []string{watchedA, watchedB, unwatchedNS} {
			_ = utils.DeleteNamespace(ctx, kubeClient, ns)
		}
	})

	It("should reload workloads across all watched namespaces but not an unwatched one", func() {
		// Set up an annotated Deployment + ConfigMap in every namespace (both
		// watched ones and the unwatched control).
		allNS := []string{watchedA, watchedB, unwatchedNS}
		for _, ns := range allNS {
			By("Creating a ConfigMap and Deployment in " + ns)
			_, err := utils.CreateConfigMap(ctx, kubeClient, ns, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = utils.CreateDeployment(ctx, kubeClient, ns, deploymentName,
				utils.WithConfigMapEnvFrom(configMapName),
				utils.WithAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(adapter.WaitReady(ctx, ns, deploymentName, utils.WorkloadReadyTimeout)).To(Succeed())
		}

		// Capture reload baselines before triggering, then update every ConfigMap.
		priorReload := map[string]string{}
		for _, ns := range allNS {
			pv, err := adapter.GetPodTemplateAnnotation(ctx, ns, deploymentName, utils.AnnotationLastReloadedFrom)
			Expect(err).NotTo(HaveOccurred())
			priorReload[ns] = pv
		}
		for _, ns := range allNS {
			By("Updating the ConfigMap in " + ns)
			Expect(utils.UpdateConfigMap(ctx, kubeClient, ns, configMapName,
				map[string]string{"key": "updated"})).To(Succeed())
		}

		By("Verifying workloads in BOTH watched namespaces reloaded")
		for _, ns := range []string{watchedA, watchedB} {
			reloaded, err := adapter.WaitReloadedFrom(ctx, ns, deploymentName,
				utils.AnnotationLastReloadedFrom, priorReload[ns], utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Deployment in watched namespace %s should reload", ns)
		}

		By("Verifying the workload in the unwatched namespace did NOT reload")
		reloaded, err := adapter.WaitReloadedFrom(ctx, unwatchedNS, deploymentName,
			utils.AnnotationLastReloadedFrom, priorReload[unwatchedNS], utils.ShortTimeout)
		Expect(err).NotTo(HaveOccurred())
		Expect(reloaded).To(BeFalse(), "Deployment in unwatched namespace %s should NOT reload", unwatchedNS)
	})
})
