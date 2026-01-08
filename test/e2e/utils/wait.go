package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// Timeout and interval constants for polling operations.
const (
	DefaultTimeout   = 30 * time.Second // General operations
	DefaultInterval  = 1 * time.Second  // Polling interval (faster feedback)
	ShortTimeout     = 5 * time.Second  // Quick checks
	NegativeTestWait = 3 * time.Second  // Wait before checking negative conditions
	DeploymentReady  = 60 * time.Second // Workload readiness (buffer for CI)
	ReloadTimeout    = 15 * time.Second // Time for reload to trigger
)

// WaitForDeploymentReady waits for a deployment to have all replicas available.
func WaitForDeploymentReady(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		deploy, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // Keep polling
		}

		// Check if deployment is ready
		if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas &&
			deploy.Status.UpdatedReplicas == *deploy.Spec.Replicas &&
			deploy.Status.AvailableReplicas == *deploy.Spec.Replicas {
			return true, nil
		}

		return false, nil
	})
}

// WaitForDeploymentReloaded waits for a deployment's pod template to have the reloader annotation.
// Returns true if the annotation was found, false if timeout occurred.
func WaitForDeploymentReloaded(ctx context.Context, client kubernetes.Interface, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		deploy, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // Keep polling
		}

		// Check pod template annotations
		if deploy.Spec.Template.Annotations != nil {
			if _, ok := deploy.Spec.Template.Annotations[annotationKey]; ok {
				found = true
				return true, nil
			}
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}

// WaitForDaemonSetReloaded waits for a DaemonSet's pod template to have the reloader annotation.
func WaitForDaemonSetReloaded(ctx context.Context, client kubernetes.Interface, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		ds, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if ds.Spec.Template.Annotations != nil {
			if _, ok := ds.Spec.Template.Annotations[annotationKey]; ok {
				found = true
				return true, nil
			}
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}

// WaitForStatefulSetReloaded waits for a StatefulSet's pod template to have the reloader annotation.
func WaitForStatefulSetReloaded(ctx context.Context, client kubernetes.Interface, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		ss, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if ss.Spec.Template.Annotations != nil {
			if _, ok := ss.Spec.Template.Annotations[annotationKey]; ok {
				found = true
				return true, nil
			}
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}

// WaitForCronJobReloaded waits for a CronJob's pod template to have the reloader annotation.
func WaitForCronJobReloaded(ctx context.Context, client kubernetes.Interface, namespace, name, annotationKey string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		cj, err := client.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if cj.Spec.JobTemplate.Spec.Template.Annotations != nil {
			if _, ok := cj.Spec.JobTemplate.Spec.Template.Annotations[annotationKey]; ok {
				found = true
				return true, nil
			}
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}

// WaitForJobCreated waits for a Job to be created with the given label selector.
func WaitForJobCreated(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		jobs, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return false, nil
		}

		if len(jobs.Items) > 0 {
			found = true
			return true, nil
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}

// WaitForCronJobTriggeredJob waits for a Job to be created by the specified CronJob.
// It checks owner references to find Jobs created by Reloader's manual trigger.
func WaitForCronJobTriggeredJob(ctx context.Context, client kubernetes.Interface, namespace, cronJobName string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		jobs, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, nil
		}

		for _, job := range jobs.Items {
			// Check if this job is owned by the CronJob
			for _, ownerRef := range job.OwnerReferences {
				if ownerRef.Kind == "CronJob" && ownerRef.Name == cronJobName {
					// Check for the manual instantiate annotation (added by Reloader)
					if job.Annotations != nil {
						if _, ok := job.Annotations["cronjob.kubernetes.io/instantiate"]; ok {
							found = true
							return true, nil
						}
					}
				}
			}
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}

// WaitForDeploymentEnvVar waits for a deployment's containers to have an environment variable
// with the given prefix (e.g., "STAKATER_").
func WaitForDeploymentEnvVar(ctx context.Context, client kubernetes.Interface, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		deploy, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if hasEnvVarWithPrefix(deploy.Spec.Template.Spec.Containers, prefix) {
			found = true
			return true, nil
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}

// WaitForDaemonSetEnvVar waits for a DaemonSet's containers to have an environment variable
// with the given prefix.
func WaitForDaemonSetEnvVar(ctx context.Context, client kubernetes.Interface, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		ds, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if hasEnvVarWithPrefix(ds.Spec.Template.Spec.Containers, prefix) {
			found = true
			return true, nil
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}

// WaitForStatefulSetEnvVar waits for a StatefulSet's containers to have an environment variable
// with the given prefix.
func WaitForStatefulSetEnvVar(ctx context.Context, client kubernetes.Interface, namespace, name, prefix string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		ss, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if hasEnvVarWithPrefix(ss.Spec.Template.Spec.Containers, prefix) {
			found = true
			return true, nil
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}

// WaitForDeploymentPaused waits for a deployment to have the paused-at annotation.
func WaitForDeploymentPaused(ctx context.Context, client kubernetes.Interface, namespace, name, pausedAtAnnotation string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		deploy, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		// Check deployment annotations (not pod template)
		if deploy.Annotations != nil {
			if _, ok := deploy.Annotations[pausedAtAnnotation]; ok {
				found = true
				return true, nil
			}
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return found, nil
}

// WaitForDeploymentUnpaused waits for a deployment to NOT have the paused-at annotation.
func WaitForDeploymentUnpaused(ctx context.Context, client kubernetes.Interface, namespace, name, pausedAtAnnotation string, timeout time.Duration) (bool, error) {
	var unpaused bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		deploy, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		// Check if paused-at annotation is gone
		if deploy.Annotations == nil {
			unpaused = true
			return true, nil
		}
		if _, ok := deploy.Annotations[pausedAtAnnotation]; !ok {
			unpaused = true
			return true, nil
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return unpaused, nil
}

// WaitForDaemonSetReady waits for a DaemonSet to have all pods ready.
func WaitForDaemonSetReady(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		ds, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if ds.Status.DesiredNumberScheduled > 0 &&
			ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
			return true, nil
		}

		return false, nil
	})
}

// WaitForStatefulSetReady waits for a StatefulSet to have all replicas ready.
func WaitForStatefulSetReady(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		ss, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
			return true, nil
		}

		return false, nil
	})
}

// GetDeployment retrieves a deployment by name.
func GetDeployment(ctx context.Context, client kubernetes.Interface, namespace, name string) (*appsv1.Deployment, error) {
	return client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetDaemonSet retrieves a DaemonSet by name.
func GetDaemonSet(ctx context.Context, client kubernetes.Interface, namespace, name string) (*appsv1.DaemonSet, error) {
	return client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetStatefulSet retrieves a StatefulSet by name.
func GetStatefulSet(ctx context.Context, client kubernetes.Interface, namespace, name string) (*appsv1.StatefulSet, error) {
	return client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetCronJob retrieves a CronJob by name.
func GetCronJob(ctx context.Context, client kubernetes.Interface, namespace, name string) (*batchv1.CronJob, error) {
	return client.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
}

// WaitForCronJobExists waits for a CronJob to exist in the cluster.
// This is useful for giving Reloader time to detect and index the CronJob before making changes.
func WaitForCronJobExists(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := client.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // Keep polling
		}
		return true, nil
	})
}

// GetJob retrieves a Job by name.
func GetJob(ctx context.Context, client kubernetes.Interface, namespace, name string) (*batchv1.Job, error) {
	return client.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
}

// hasEnvVarWithPrefix checks if any container has an environment variable with the given prefix.
func hasEnvVarWithPrefix(containers []corev1.Container, prefix string) bool {
	for _, container := range containers {
		for _, env := range container.Env {
			if strings.HasPrefix(env.Name, prefix) {
				return true
			}
		}
	}
	return false
}

// WaitForJobRecreated waits for a Job to be deleted and recreated with a new UID.
// Returns the new Job's UID if recreation was detected.
func WaitForJobRecreated(ctx context.Context, client kubernetes.Interface, namespace, name, originalUID string, timeout time.Duration) (string, bool, error) {
	var newUID string
	var recreated bool

	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		job, err := client.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			// Job not found means it's been deleted, keep polling for recreation
			return false, nil
		}

		// Check if the UID has changed (indicating recreation)
		if string(job.UID) != originalUID {
			newUID = string(job.UID)
			recreated = true
			return true, nil
		}

		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return "", false, err
	}
	return newUID, recreated, nil
}

// WaitForJobNotFound waits for a Job to be deleted.
func WaitForJobNotFound(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) (bool, error) {
	var deleted bool

	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := client.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			deleted = true
			return true, nil
		}
		return false, nil
	})

	if err != nil && err != context.DeadlineExceeded {
		return false, err
	}
	return deleted, nil
}

// WaitForJobExists waits for a Job to exist in the cluster.
func WaitForJobExists(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := client.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil // Keep polling
		}
		return true, nil
	})
}

// GetPodLogs retrieves logs from pods matching the given label selector.
func GetPodLogs(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string) (string, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods: %w", err)
	}

	var allLogs strings.Builder
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			logs, err := client.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				Container: container.Name,
			}).Do(ctx).Raw()
			if err != nil {
				allLogs.WriteString(fmt.Sprintf("Error getting logs for %s/%s: %v\n", pod.Name, container.Name, err))
				continue
			}
			allLogs.WriteString(fmt.Sprintf("=== %s/%s ===\n%s\n", pod.Name, container.Name, string(logs)))
		}
	}

	return allLogs.String(), nil
}
