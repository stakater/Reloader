module github.com/stakater/Reloader

go 1.21

require (
	github.com/argoproj/argo-rollouts v1.5.1
	github.com/openshift/api v3.9.0+incompatible
	github.com/openshift/client-go v0.0.0-20230807132528-be5346fb33cb
	github.com/parnurzeal/gorequest v0.2.16
	github.com/prometheus/client_golang v1.16.0
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.7.0
	k8s.io/api v0.28.3
	k8s.io/apimachinery v0.28.3
	k8s.io/client-go v0.28.3
	k8s.io/kubectl v0.28.3
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/elazarl/goproxy v0.0.0-20221015165544-a0805db90819 // indirect
	github.com/emicklei/go-restful/v3 v3.10.1 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.42.0 // indirect
	github.com/prometheus/procfs v0.11.1 // indirect
	github.com/smartystreets/goconvey v1.7.2 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/oauth2 v0.8.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/term v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230717233707-2695361300d9 // indirect
	moul.io/http2curl v1.0.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

// Replacements for argo-rollouts
replace (
	github.com/go-check/check => github.com/go-check/check v0.0.0-20201130134442-10cb98267c6c
	k8s.io/api v0.0.0 => k8s.io/api v0.27.4
	k8s.io/apimachinery v0.0.0 => k8s.io/apimachinery v0.27.4
	k8s.io/client-go v0.0.0 => k8s.io/client-go v0.27.4
	k8s.io/cloud-provider v0.0.0 => k8s.io/cloud-provider v0.24.2
	k8s.io/controller-manager v0.0.0 => k8s.io/controller-manager v0.24.2
	k8s.io/cri-api v0.0.0 => k8s.io/cri-api v0.20.5-rc.0
	k8s.io/csi-translation-lib v0.0.0 => k8s.io/csi-translation-lib v0.24.2
	k8s.io/kube-aggregator v0.0.0 => k8s.io/kube-aggregator v0.24.2
	k8s.io/kube-controller-manager v0.0.0 => k8s.io/kube-controller-manager v0.24.2
	k8s.io/kube-proxy v0.0.0 => k8s.io/kube-proxy v0.24.2
	k8s.io/kube-scheduler v0.0.0 => k8s.io/kube-scheduler v0.24.2
	k8s.io/kubectl v0.0.0 => k8s.io/kubectl v0.27.1
	k8s.io/kubelet v0.0.0 => k8s.io/kubelet v0.24.2
	k8s.io/legacy-cloud-providers v0.0.0 => k8s.io/legacy-cloud-providers v0.24.2
	k8s.io/mount-utils v0.0.0 => k8s.io/mount-utils v0.20.5-rc.0
	k8s.io/sample-apiserver v0.0.0 => k8s.io/sample-apiserver v0.24.2
	k8s.io/sample-cli-plugin v0.0.0 => k8s.io/sample-cli-plugin v0.24.2
	k8s.io/sample-controller v0.0.0 => k8s.io/sample-controller v0.24.2
)
