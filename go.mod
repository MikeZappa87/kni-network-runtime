module github.com/mikezappa87/kni-network-runtime

go 1.21.5

require (
	github.com/MikeZappa87/kni-api v0.0.6
	github.com/containerd/cri-containerd v1.19.0
	github.com/containerd/go-cni v1.1.9
	github.com/fsnotify/fsnotify v1.7.0
	github.com/sirupsen/logrus v1.9.3
	go.etcd.io/bbolt v1.3.8
	google.golang.org/grpc v1.58.3
	k8s.io/apimachinery v0.29.1
)

require (
	github.com/containernetworking/cni v1.1.2 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/klog/v2 v2.110.1 // indirect
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b // indirect
)

replace github.com/containerd/go-cni => github.com/mikezappa87/go-cni v1.1.1-0.20240114032345-d4b1b5a94b43
