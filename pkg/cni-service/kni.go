package cniservice

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MikeZappa87/kni-api/pkg/apis/runtime/beta"
	"github.com/containerd/go-cni"
	"github.com/mikezappa87/kni-network-runtime/pkg/netns"
	log "github.com/sirupsen/logrus"
)

type KNIConfig struct {
	UseMultiNet bool
	IfPrefix    string
	Db          string
}

type KniService struct {
	c      cni.CNI
	store  *Store
	config KNIConfig
}

func NewKniService(config *KNIConfig) (beta.KNIServer, error) {
	log.Info("starting kni network runtime service")

	opts := []cni.Opt{
		cni.WithLoNetwork,
		cni.WithAllConf}

	initopts := []cni.Opt{
		cni.WithMinNetworkCount(2),
		cni.WithInterfacePrefix(config.IfPrefix),
	}

	cni, err := cni.New(initopts...)

	if err != nil {
		return nil, err
	}

	db, err := New(config.Db)

	if err != nil {
		return nil, err
	}

	kni := &KniService{
		c:      cni,
		store:  db,
		config: *config,
	}

	sync, err := newCNINetConfSyncer("/etc/cni/net.d", cni, opts)

	if err != nil {
		return nil, err
	}

	go func() {
		sync.syncLoop()
	}()

	log.Info("cni has been loaded")

	return kni, nil
}

func (k *KniService) CreateNetwork(ctx context.Context, req *beta.CreateNetworkRequest) (*beta.CreateNetworkResponse, error) {
	ns, err := netns.NewNetNS(fmt.Sprintf("/var/run/netns/kni-%s-%s", req.Namespace, req.Name))
	
	if err != nil {
		return nil, err
	}

	return &beta.CreateNetworkResponse{
		NetnsPath: ns.GetPath(),
	}, nil 
}

func (k *KniService) DeleteNetwork(ctx context.Context, req *beta.DeleteNetworkRequest) (*beta.DeleteNetworkResponse, error) {
	path := fmt.Sprintf("/var/run/netns/kni-%s-%s", req.Namespace, req.Name)

	ns := netns.LoadNetNS(path)
	err := ns.Remove()

	if err != nil {
		return nil, err
	}

	return &beta.DeleteNetworkResponse{}, nil
}

func (k *KniService) AttachNetwork(ctx context.Context, req *beta.AttachNetworkRequest) (*beta.AttachNetworkResponse, error) {
	log.Infof("attach rpc request for id %s", req.Id)

	opts, err := cniNamespaceOpts(req.Id, req.Name, req.Namespace, "", req.Labels,
	 req.Annotations, req.Extradata, req.PortMappings, req.DnsConfig)

	 if err != nil {
		return nil, err
	}

	if _, ok := req.Extradata["netns"]; !ok {
		return nil, fmt.Errorf("pod annotation id: %s has no netns", req.Id)
	}

	netns := req.Extradata["netns"]

	var res *cni.Result

	if k.config.UseMultiNet {
		res, err = k.SetupMultipleNetworks(ctx, req, netns)

		if err != nil {
			log.Errorf("unable to execute CNI ADD: %s", err.Error())

			return nil, err
		}
	} else {
		res, err = k.c.SetupSerially(ctx, req.Id, netns, opts...)

		if err != nil {
			log.Errorf("unable to execute CNI ADD: %s", err.Error())

			return nil, err
		}
	}

	cniResult, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}

	log.Infof("CNI Result: %s", string(cniResult))

	store := NetworkStorage{
		IP:          make(map[string]*beta.IPConfig),
		Annotations: req.Annotations,
		Extradata:   req.Extradata,
	}

	for outk, outv := range res.Interfaces {
		data := &beta.IPConfig{}
		store.IP[outk] = data
		data.Mac = outv.Mac

		for _, v := range outv.IPConfigs {
			data.Ip = append(data.Ip, v.IP.String())
		}
	}

	log.WithField("ipconfigs", store.IP).Info("cni add executed")

	err = k.store.Save(req.Id, store)

	if err != nil {
		log.Errorf("unable to save record for id: %s: %s", req.Id, err.Error())
		return nil, err
	}

	return &beta.AttachNetworkResponse{
		Ipconfigs: store.IP,
	}, nil
}

func (k *KniService) DetachNetwork(ctx context.Context, req *beta.DetachNetworkRequest) (*beta.DetachNetworkResponse, error) {

	log.Infof("detach rpc request for id %s", req.Id)

	opts := []cni.NamespaceOpts{
		cni.WithArgs("IgnoreUnknown", "1"),
		cni.WithLabels(req.Labels),
		cni.WithLabels(req.Annotations),
		cni.WithLabels(req.Extradata),
	}

	query, err := k.store.Query(req.Id)

	if err != nil {
		log.Errorf("unable to query record id: %s %v", req.Id, err)

		return nil, err
	}

	if cgroup := query.Extradata["cgroupPath"]; cgroup != "" {
		opts = append(opts, cni.WithCapabilityCgroupPath(cgroup))
		log.Infof("cgroup: %s", cgroup)
	}

	netns := query.Extradata["netns"]

	if k.config.UseMultiNet {
		err = k.RemoveMultipleNetworks(ctx, req, netns)

		if err != nil {
			log.Errorf("unable to execute CNI DEL: %s", err.Error())

			return nil, err
		}
	} else {
		err = k.c.Remove(ctx, req.Id, netns, opts...)

		if err != nil {
			log.Errorf("unable to execute CNI DEL: %s", err.Error())

			return nil, err
		}
	}

	err = k.store.Delete(req.Id)

	if err != nil {
		log.Errorf("unable to delete record id: %s %v", req.Id, err)

		return nil, err
	}

	return &beta.DetachNetworkResponse{}, nil
}

func (k *KniService) SetupNodeNetwork(context.Context, *beta.SetupNodeNetworkRequest) (*beta.SetupNodeNetworkResponse, error) {
	//Setup the initial node network

	return nil, nil
}

func (k *KniService) QueryPodNetwork(ctx context.Context, req *beta.QueryPodNetworkRequest) (*beta.QueryPodNetworkResponse, error) {

	log.Infof("query pod rpc request id: %s", req.Id)

	data, err := k.store.Query(req.Id)

	if data.IP == nil {
		return &beta.QueryPodNetworkResponse{}, nil
	}

	log.Infof("ipconfigs received for id: %s ip: %s", req.Id, data.IP)

	if err != nil {
		return nil, err
	}

	return &beta.QueryPodNetworkResponse{
		Ipconfigs: data.IP,
	}, nil
}

func (k *KniService) QueryNodeNetworks(ctx context.Context, req *beta.QueryNodeNetworksRequest) (*beta.QueryNodeNetworksResponse, error) {
	networks := []*beta.Network{}

	if err := k.c.Status(); err != nil {
		networks = append(networks, &beta.Network{
			Name:      "default",
			Ready:     false,
			Extradata: map[string]string{},
		})
	} else {
		networks = append(networks, &beta.Network{
			Name:  "default",
			Ready: true,
		})
	}

	return &beta.QueryNodeNetworksResponse{
		Networks: networks,
	}, nil
}

func (k *KniService) SetupMultipleNetworks(ctx context.Context, req *beta.AttachNetworkRequest, netns string) (*cni.Result, error) {
	x := extractNetworks(req.Annotations)

	appendDefaultCNINetworks(&x, k.c)

	for _, v := range x {
		log.Infof("setting up network: %s", v.NetworkName)
	}

	bytes, err := json.Marshal(x)

	if err != nil {
		return nil, err
	}

	log.Infof("setting up network with: %s", string(bytes))

	networks, err := k.c.BuildMultiNetwork(x)

	if err != nil {
		return nil, err
	}

	opts := []cni.NamespaceOpts{}

	res, err := k.c.SetupNetworks(ctx, req.Id, netns, networks, opts...)

	if err != nil {
		return nil, err
	}

	return res, nil
}

func (k *KniService) RemoveMultipleNetworks(ctx context.Context, req *beta.DetachNetworkRequest, netns string) error {
	x := extractNetworks(req.Annotations)

	appendDefaultCNINetworks(&x, k.c)

	networks, err := k.c.BuildMultiNetwork(x)

	if err != nil {
		return err
	}

	opts := []cni.NamespaceOpts{}

	err = k.c.RemoveNetworks(ctx, req.Id, netns, networks, opts...)

	if err != nil {
		return nil
	}

	return nil
}
