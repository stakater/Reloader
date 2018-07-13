package upgrader

import (
	"bytes"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	updateOnChangeAnnotation = "reloader.stakater.com/update-on-change"
)

// Upgrader will upgrade the relevent deployment, deamonset and deamonset.
type Upgrader struct {
	client       kubernetes.Interface
	resourceType string
}

// NewUpgrader Initializes the Upgrader
func NewUpgrader(client kubernetes.Interface, resourceType string) (*Upgrader, error) {
	u := Upgrader{
		client:       client,
		resourceType: resourceType,
	}
	return &u, nil
}

// ObjectCreated Detects if the configmap or secret has been created
func (u *Upgrader) ObjectCreated(obj interface{}) {
	message := u.resourceType + ": `" + obj.(*v1.ConfigMap).Name + "`has been created in Namespace: `" + obj.(*v1.ConfigMap).Namespace + "`"
	logrus.Infof(message)
	err := rollingUpgradeDeployments(obj, u.client)
	if err != nil {
		logrus.Errorf("failed to update Deployment: %v", err)
	}
}

// ObjectUpdated Detects if the configmap or secret has been updated
func (u *Upgrader) ObjectUpdated(oldObj interface{}) {
	message := u.resourceType + ": `" + oldObj.(*v1.ConfigMap).Name + "`has been updated in Namespace: `" + oldObj.(*v1.ConfigMap).Namespace + "`"
	logrus.Infof(message)
	err := rollingUpgradeDeployments(oldObj, u.client)
	if err != nil {
		logrus.Errorf("failed to update Deployment: %v", err)
	}
}

// Implementation has been borrowed from fabric8io/configmapcontroller
// Method has been modified a little to use updated liberaries.
func rollingUpgradeDeployments(oldObj interface{}, client kubernetes.Interface) error {
	ns := oldObj.(*v1.ConfigMap).Namespace
	configMapName := oldObj.(*v1.ConfigMap).Name
	configMapVersion := convertConfigMapToToken(oldObj.(*v1.ConfigMap))

	deployments, err := client.ExtensionsV1beta1().Deployments(ns).List(meta_v1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list deployments")
	}
	for _, d := range deployments.Items {
		containers := d.Spec.Template.Spec.Containers
		// match deployments with the correct annotation
		annotationValue := d.ObjectMeta.Annotations[updateOnChangeAnnotation]
		if annotationValue != "" {
			values := strings.Split(annotationValue, ",")
			matches := false
			for _, value := range values {
				if value == configMapName {
					matches = true
					break
				}
			}
			if matches {
				updateContainers(containers, annotationValue, configMapVersion)

				// update the deployment
				_, err := client.ExtensionsV1beta1().Deployments(ns).Update(&d)
				if err != nil {
					return errors.Wrap(err, "update deployment failed")
				}
				logrus.Infof("Updated Deployment %s", d.Name)
			}
		}
	}
	return nil
}

func updateContainers(containers []v1.Container, annotationValue, configMapVersion string) bool {
	// we can have multiple configmaps to update
	answer := false
	configmaps := strings.Split(annotationValue, ",")
	for _, cmNameToUpdate := range configmaps {
		configmapEnvar := "STAKATER_" + convertToEnvVarName(cmNameToUpdate) + "_CONFIGMAP"

		for i := range containers {
			envs := containers[i].Env
			matched := false
			for j := range envs {
				if envs[j].Name == configmapEnvar {
					matched = true
					if envs[j].Value != configMapVersion {
						logrus.Infof("Updating %s to %s", configmapEnvar, configMapVersion)
						envs[j].Value = configMapVersion
						answer = true
					}
				}
			}
			// if no existing env var exists lets create one
			if !matched {
				e := v1.EnvVar{
					Name:  configmapEnvar,
					Value: configMapVersion,
				}
				containers[i].Env = append(containers[i].Env, e)
				answer = true
			}
		}
	}
	return answer
}

// convertToEnvVarName converts the given text into a usable env var
// removing any special chars with '_'
func convertToEnvVarName(text string) string {
	var buffer bytes.Buffer
	lower := strings.ToUpper(text)
	lastCharValid := false
	for i := 0; i < len(lower); i++ {
		ch := lower[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			buffer.WriteString(string(ch))
			lastCharValid = true
		} else {
			if lastCharValid {
				buffer.WriteString("_")
			}
			lastCharValid = false
		}
	}
	return buffer.String()
}

// lets convert the configmap into a unique token based on the data values
func convertConfigMapToToken(cm *v1.ConfigMap) string {
	values := []string{}
	for k, v := range cm.Data {
		values = append(values, k+"="+v)
	}
	sort.Strings(values)
	text := strings.Join(values, ";")
	// we could zip and base64 encode
	// but for now we could leave this easy to read so that its easier to diagnose when & why things changed
	return text
}
