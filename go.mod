module github.com/stakater/Reloader

go 1.13

require (
	github.com/golang/groupcache v0.0.0-20191002201903-404acd9df4cc // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/onsi/ginkgo v1.10.2 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/openshift/api v3.9.1-0.20190923092516-169848dd8137+incompatible
	github.com/openshift/client-go v0.0.0-20190923092832-6afefc9bb372
	github.com/prometheus/client_golang v1.4.1
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v0.0.0-20160722081547-f62e98d28ab7
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	k8s.io/api v0.0.0-20190918155943-95b840bb6a1f
	k8s.io/apimachinery v0.0.0-20191004115801-a2eda9f80ab8
	k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90
)

replace (
	github.com/openshift/api => github.com/openshift/api v3.9.1-0.20190923092516-169848dd8137+incompatible // prebase-1.16
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20190923092832-6afefc9bb372 // prebase-1.16
	k8s.io/api => k8s.io/api v0.0.0-20191004120104-195af9ec3521 // release-1.16
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20191004115801-a2eda9f80ab8 // kubernetes-1.16.0
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90 // kubernetes-1.16.0
)
