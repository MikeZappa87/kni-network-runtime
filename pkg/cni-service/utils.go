package cniservice

import (
	"strings"

	"github.com/containerd/go-cni"
)

func appendDefaultCNINetworks(net *[]*cni.NetworkInterface, plugin cni.CNI) {
	*net = append(*net, &cni.NetworkInterface{
		NetworkName:   "cni-loopback",
		InterfaceName: "lo",
	},
		&cni.NetworkInterface{
			InterfaceName: "eth0",
			//Index 0 and 1 should always be here
			NetworkName: plugin.GetConfig().Networks[1].Config.Name,
		})
}

func extractNetworks(annotations map[string]string) []*cni.NetworkInterface {
	var x []*cni.NetworkInterface

	if val, ok := annotations["kni.io/multi-network"]; ok {
		for _, value := range strings.Split(val, ",") {
			if strings.Contains(value, "@") {
				a := strings.Split(value, "@")

				x = append(x, &cni.NetworkInterface{
					NetworkName:   a[0],
					InterfaceName: a[1],
				})
			} else {
				x = append(x, &cni.NetworkInterface{
					NetworkName: value,
				})
			}
		}
	}

	return x
}
