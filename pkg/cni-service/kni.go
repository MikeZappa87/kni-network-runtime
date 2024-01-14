package cniservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/MikeZappa87/kni-api/pkg/apis/runtime/beta"
	"github.com/containerd/go-cni"
	log "github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

type KNIConfig struct {
	UseMultiNet bool
	IfPrefix    string
	Db          string
}

type KniService struct {
	c      cni.CNI
	store  *bolt.DB
	config KNIConfig
}

type attachStore struct {
	IP          map[string]*beta.IPConfig
	Annotations map[string]string
}

const PodBucket = "pod"

func NewKniService(config *KNIConfig) (beta.KNIServer, error) {
	log.Info("starting kni network runtime service")

	opts := []cni.Opt{
		cni.WithInterfacePrefix(config.IfPrefix),
		cni.WithAllConf,
		cni.WithLoNetwork}

	cni, err := cni.New()

	if err != nil {
		return nil, err
	}

	db, err := bolt.Open(config.Db, 0600, nil)
	if err != nil {
		return nil, err
	}

	log.Info("boltdb connection is open")

	kni := &KniService{
		c:      cni,
		store:  db,
		config: *config,
	}

	db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte(PodBucket))

		return nil
	})

	kni.c.Load(opts...)

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

func (k *KniService) AttachNetwork(ctx context.Context, req *beta.AttachNetworkRequest) (*beta.AttachNetworkResponse, error) {
	//This is nice. In the container runtime if you want to add one you need to contribute this to the container runtime
	//This way you are in complete control. Better capability support with KNI -> CNI

	log.Infof("attach rpc request for id %s", req.Id)

	opts := []cni.NamespaceOpts{
		cni.WithArgs("IgnoreUnknown", "1"),
		cni.WithLabels(req.Labels),
		cni.WithLabels(req.Annotations),
		cni.WithLabels(req.Extradata),
	}

	if req.Isolation == nil {
		return &beta.AttachNetworkResponse{}, nil
	}

	if req.Isolation.Path != "" {
		log.Infof("pod annotation id: %s netns: %s\n", req.Id, req.Isolation.Path)
	} else {
		log.Info("no network namespace path, this is either a bug or its running in the root")
		return &beta.AttachNetworkResponse{}, nil
	}

	var res *cni.Result
	var err error

	if k.config.UseMultiNet {
		res, err = k.SetupMultipleNetworks(ctx, req)

		if err != nil {
			log.Errorf("unable to execute CNI ADD: %s", err.Error())

			return nil, err
		}
	} else {
		res, err = k.c.SetupSerially(ctx, req.Id, req.Isolation.Path, opts...)

		if err != nil {
			log.Errorf("unable to execute CNI ADD: %s", err.Error())

			return nil, err
		}
	}

	store := attachStore{
		IP:          make(map[string]*beta.IPConfig),
		Annotations: map[string]string{},
	}

	ip := make(map[string]*beta.IPConfig)

	for outk, outv := range res.Interfaces {
		data := &beta.IPConfig{}
		ip[outk] = data
		data.Mac = outv.Mac

		for _, v := range outv.IPConfigs {
			data.Ip = append(data.Ip, v.IP.String())
		}
	}

	store.Annotations = req.Annotations
	store.IP = ip

	log.WithField("ipconfigs", ip).Info("cni add executed")

	err = k.store.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(PodBucket))
		if b == nil {
			return fmt.Errorf("bucket does not exist")
		}

		if err != nil {
			return err
		}

		js, err := json.Marshal(store)

		if err != nil {
			return err
		}

		return b.Put([]byte(req.Id), js)
	})

	if err != nil {
		log.Errorf("unable to save record for id: %s: %s", req.Id, err.Error())
		return nil, err
	}

	return &beta.AttachNetworkResponse{
		Ipconfigs: ip,
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

	err := k.store.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PodBucket))

		if b == nil {
			return errors.New("bucket not created")
		}

		v := b.Get([]byte(req.Id))

		if v == nil {
			return nil
		}

		data := attachStore{}

		err := json.Unmarshal(v, &data)

		if err != nil {
			return err
		}

		if k.config.UseMultiNet {
			err = k.RemoveMultipleNetworks(ctx, req)

			if err != nil {
				log.Errorf("unable to execute CNI DEL: %s", err.Error())

				return err
			}
		} else {
			err = k.c.Remove(ctx, req.Id, data.Annotations["netns"], opts...)

			if err != nil {
				log.Errorf("unable to execute CNI DEL: %s", err.Error())

				return err
			}
		}

		return nil
	})

	if err != nil {
		log.Errorf("unable to execute CNI DEL: %s %v", req.Id, err)

		return nil, err
	}

	err = k.store.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(PodBucket))

		if err != nil {
			return err
		}

		return b.Delete([]byte(req.Id))
	})

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

	data := attachStore{}

	err := k.store.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(PodBucket))

		if b == nil {
			return errors.New("bucket not created")
		}

		v := b.Get([]byte(req.Id))

		if v == nil {
			return nil
		}

		err := json.Unmarshal(v, &data)

		if err != nil {
			return err
		}

		return nil
	})

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

func (k *KniService) SetupMultipleNetworks(ctx context.Context, req *beta.AttachNetworkRequest) (*cni.Result, error) {
	x := extractNetworks(req.Annotations)

	appendDefaultCNINetworks(&x, k.c)

	networks, err := k.c.BuildMultiNetwork(x)

	if err != nil {
		return nil, err
	}

	opts := []cni.NamespaceOpts{}

	res, err := k.c.SetupNetworks(ctx, req.Id, req.Isolation.Path, networks, opts...)

	if err != nil {
		return nil, err
	}

	return res, nil
}

func (k *KniService) RemoveMultipleNetworks(ctx context.Context, req *beta.DetachNetworkRequest) error {
	x := extractNetworks(req.Annotations)

	appendDefaultCNINetworks(&x, k.c)

	networks, err := k.c.BuildMultiNetwork(x)

	if err != nil {
		return err
	}

	opts := []cni.NamespaceOpts{}

	err = k.c.RemoveNetworks(ctx, req.Id, req.Annotations["netns"], networks, opts...)

	if err != nil {
		return nil
	}

	return nil
}
