module github.com/stakater/Reloader

go 1.26.2

require (
	github.com/argoproj/argo-rollouts v1.9.0
	github.com/openshift/api v0.0.0-20260420151639-34e60874783e
	github.com/openshift/client-go v0.0.0-20260330134249-7e1499aaacd7
	github.com/parnurzeal/gorequest v0.3.0
	github.com/prometheus/client_golang v1.23.2
	github.com/sirupsen/logrus v1.9.4
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.11.1
	k8s.io/api v0.35.3
	k8s.io/apimachinery v0.35.3
	k8s.io/client-go v0.35.3
	k8s.io/kubectl v0.35.3
	sigs.k8s.io/secrets-store-csi-driver v1.5.5
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/elazarl/goproxy v0.0.0-20240726154733-8b0c20506380 // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/fxamacker/cbor/v2 v2.9.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-openapi/jsonpointer v0.22.5 // indirect
	github.com/go-openapi/jsonreference v0.21.5 // indirect
	github.com/go-openapi/swag v0.25.5 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.5 // indirect
	github.com/go-openapi/swag/conv v0.25.5 // indirect
	github.com/go-openapi/swag/fileutils v0.25.5 // indirect
	github.com/go-openapi/swag/jsonname v0.25.5 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.5 // indirect
	github.com/go-openapi/swag/loading v0.25.5 // indirect
	github.com/go-openapi/swag/mangling v0.25.5 // indirect
	github.com/go-openapi/swag/netutils v0.25.5 // indirect
	github.com/go-openapi/swag/stringutils v0.25.5 // indirect
	github.com/go-openapi/swag/typeutils v0.25.5 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gnostic-models v0.7.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/moul/http2curl v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/smartystreets/goconvey v1.7.2 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/term v0.41.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20260330154417-16be699c7b31 // indirect
	k8s.io/utils v0.0.0-20260319190234-28399d86e0b5 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.2 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

// Replacements for argo-rollouts
replace (
	github.com/go-check/check => github.com/go-check/check v0.0.0-20201130134442-10cb98267c6c
	k8s.io/api v0.0.0 => k8s.io/api v0.35.3
	k8s.io/apimachinery v0.0.0 => k8s.io/apimachinery v0.35.3
	k8s.io/client-go v0.0.0 => k8s.io/client-go v0.35.3
	k8s.io/cloud-provider v0.0.0 => k8s.io/cloud-provider v0.24.2
	k8s.io/controller-manager v0.0.0 => k8s.io/controller-manager v0.35.3
	k8s.io/cri-api v0.0.0 => k8s.io/cri-api v0.20.5-rc.0
	k8s.io/csi-translation-lib v0.0.0 => k8s.io/csi-translation-lib v0.24.2
	k8s.io/kube-aggregator v0.0.0 => k8s.io/kube-aggregator v0.24.2
	k8s.io/kube-controller-manager v0.0.0 => k8s.io/kube-controller-manager v0.24.2
	k8s.io/kube-proxy v0.0.0 => k8s.io/kube-proxy v0.24.2
	k8s.io/kube-scheduler v0.0.0 => k8s.io/kube-scheduler v0.24.2
	k8s.io/kubectl v0.0.0 => k8s.io/kubectl v0.35.3
	k8s.io/kubelet v0.0.0 => k8s.io/kubelet v0.35.3
	k8s.io/legacy-cloud-providers v0.0.0 => k8s.io/legacy-cloud-providers v0.24.2
	k8s.io/mount-utils v0.0.0 => k8s.io/mount-utils v0.35.3
	k8s.io/sample-apiserver v0.0.0 => k8s.io/sample-apiserver v0.24.2
	k8s.io/sample-cli-plugin v0.0.0 => k8s.io/sample-cli-plugin v0.24.2
	k8s.io/sample-controller v0.0.0 => k8s.io/sample-controller v0.24.2
)
