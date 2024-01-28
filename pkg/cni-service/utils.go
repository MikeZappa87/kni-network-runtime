package cniservice

import (
	"fmt"
	"math"
	"strings"

	"github.com/MikeZappa87/kni-api/pkg/apis/runtime/beta"
	"github.com/containerd/cri-containerd/pkg/server/bandwidth"
	"github.com/containerd/go-cni"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"
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

func cniNamespaceOpts(id, name, namespace, uid string, labels map[string]string, annotations map[string]string, extra map[string]string,
	 ports []*beta.PortMapping, dnsconfig *beta.DNSConfig) ([]cni.NamespaceOpts, error) {
	opts := []cni.NamespaceOpts{
		cni.WithLabels(toCNILabels(id, name, namespace, uid)),
		cni.WithCapability("io.kubernetes.cri.pod-annotations", annotations),
	}

	portMappings := toCNIPortMappings(ports)
	if len(portMappings) > 0 {
		opts = append(opts, cni.WithCapabilityPortMap(portMappings))
	}

	// Will return an error if the bandwidth limitation has the wrong unit
	// or an unreasonable value see validateBandwidthIsReasonable()
	bandWidth, err := toCNIBandWidth(annotations)
	if err != nil {
		return nil, err
	}
	if bandWidth != nil {
		opts = append(opts, cni.WithCapabilityBandWidth(*bandWidth))
	}

	dns := toCNIDNS(dnsconfig)
	if dns != nil {
		opts = append(opts, cni.WithCapabilityDNS(*dns))
	}

	if cgroup := extra["cgroupPath"]; cgroup != "" {
		opts = append(opts, cni.WithCapabilityCgroupPath(cgroup))
		log.Infof("cgroup: %s", cgroup)
	}

	return opts, nil
}

func toCNILabels(id, name, namespace, uid string) map[string]string {
	return map[string]string{
		"K8S_POD_NAMESPACE":          namespace,
		"K8S_POD_NAME":               name,
		"K8S_POD_INFRA_CONTAINER_ID": id,
		"K8S_POD_UID":                uid,
		"IgnoreUnknown":              "1",
	}
}

var minRsrc = resource.MustParse("1k")
var maxRsrc = resource.MustParse("1P")

func validateBandwidthIsReasonable(rsrc *resource.Quantity) error {
	if rsrc.Value() < minRsrc.Value() {
		return fmt.Errorf("resource is unreasonably small (< 1kbit)")
	}
	if rsrc.Value() > maxRsrc.Value() {
		return fmt.Errorf("resource is unreasonably large (> 1Pbit)")
	}
	return nil
}

// ExtractPodBandwidthResources extracts the ingress and egress from the given pod annotations
func ExtractPodBandwidthResources(podAnnotations map[string]string) (ingress, egress *resource.Quantity, err error) {
	if podAnnotations == nil {
		return nil, nil, nil
	}
	str, found := podAnnotations["kubernetes.io/ingress-bandwidth"]
	if found {
		ingressValue, err := resource.ParseQuantity(str)
		if err != nil {
			return nil, nil, err
		}
		ingress = &ingressValue
		if err := validateBandwidthIsReasonable(ingress); err != nil {
			return nil, nil, err
		}
	}
	str, found = podAnnotations["kubernetes.io/egress-bandwidth"]
	if found {
		egressValue, err := resource.ParseQuantity(str)
		if err != nil {
			return nil, nil, err
		}
		egress = &egressValue
		if err := validateBandwidthIsReasonable(egress); err != nil {
			return nil, nil, err
		}
	}
	return ingress, egress, nil
}


// toCNIBandWidth converts CRI annotations to CNI bandwidth.
func toCNIBandWidth(annotations map[string]string) (*cni.BandWidth, error) {
	ingress, egress, err := bandwidth.ExtractPodBandwidthResources(annotations)
	if err != nil {
		return nil, fmt.Errorf("reading pod bandwidth annotations: %w", err)
	}

	if ingress == nil && egress == nil {
		return nil, nil
	}

	bandWidth := &cni.BandWidth{}

	if ingress != nil {
		bandWidth.IngressRate = uint64(ingress.Value())
		bandWidth.IngressBurst = math.MaxUint32
	}

	if egress != nil {
		bandWidth.EgressRate = uint64(egress.Value())
		bandWidth.EgressBurst = math.MaxUint32
	}

	return bandWidth, nil
}

// toCNIPortMappings converts CRI port mappings to CNI.
func toCNIPortMappings(criPortMappings []*beta.PortMapping) []cni.PortMapping {
	var portMappings []cni.PortMapping
	for _, mapping := range criPortMappings {
		if mapping.HostPort <= 0 {
			continue
		}
		portMappings = append(portMappings, cni.PortMapping{
			HostPort:      mapping.HostPort,
			ContainerPort: mapping.ContainerPort,
			Protocol:      strings.ToLower(mapping.Protocol.String()),
			HostIP:        mapping.HostIp,
		})
	}
	return portMappings
}

// toCNIDNS converts CRI DNSConfig to CNI.
func toCNIDNS(dns *beta.DNSConfig) *cni.DNS {
	if dns == nil {
		return nil
	}
	return &cni.DNS{
		Servers:  dns.GetServers(),
		Searches: dns.GetSearches(),
		Options:  dns.GetOptions(),
	}
}
