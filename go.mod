module github.com/mikezappa87/kni-network-runtime

go 1.21.5

require (
	github.com/MikeZappa87/kni-api v0.0.7-0.20240201220129-cdea46f36b48
	github.com/containerd/cri-containerd v1.19.0
	github.com/containerd/go-cni v1.1.9
	github.com/fsnotify/fsnotify v1.7.0
	github.com/sirupsen/logrus v1.9.3
	go.etcd.io/bbolt v1.3.8
	google.golang.org/grpc v1.58.3
	k8s.io/apimachinery v0.29.1
)

require (
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/Microsoft/hcsshim v0.12.0-rc.0 // indirect
	github.com/containerd/cgroups/v3 v3.0.2 // indirect
	github.com/containerd/containerd v1.7.8 // indirect
	github.com/containerd/containerd/v2 v2.0.0-beta.0 // indirect
	github.com/containerd/continuity v0.4.2 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containernetworking/cni v1.1.2 // indirect
	github.com/containernetworking/plugins v1.4.0 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/sys/symlink v0.2.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools v0.16.1 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/klog/v2 v2.110.1 // indirect
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b // indirect
)

replace github.com/containerd/go-cni => github.com/mikezappa87/go-cni v1.1.1-0.20240114032345-d4b1b5a94b43
