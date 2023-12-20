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

type KniService struct {
	c cni.CNI
	store *bolt.DB
}

const PodBucket = "pod"

func NewKniService(ifprefix, dbname string) (beta.KNIServer, error) {
	log.Info("starting kni network runtime service")
	
	opts := []cni.Opt{
		cni.WithInterfacePrefix(ifprefix),
		 cni.WithDefaultConf,
		 cni.WithLoNetwork} 
	
	cni, err := cni.New()

	if err != nil {
		return nil, err
	}
	
	db, err := bolt.Open(dbname, 0600, nil)
	if err != nil {
  		return nil, err
	}

	log.Info("boltdb connection is open")

	kni := &KniService{
		c: cni,
		store: db,
	}

	db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte(PodBucket))

		return nil
	})

	err = kni.c.Load(opts...)

	log.Info("cni has been loaded")

	if err != nil {
		return nil, err
	}

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
		cni.WithLabels(req.Metadata),
	}

	if req.Isolation == nil {
		return &beta.AttachNetworkResponse{}, nil
	}

	if req.Isolation.Path != "" {
		log.Infof("pod annotation id: %s netns: %s\n", req.Id, req.Isolation.Path)
	} else {
		log.Info("no network namespace path, this is either a bug or its running in the root")
		return &beta.AttachNetworkResponse{
		}, nil
	}
	
	res, err := k.c.SetupSerially(ctx, req.Id, req.Isolation.Path, opts...)

	if err != nil {
		log.Errorf("unable to execute CNI ADD: %s",err.Error())

		return nil, err
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

	log.WithField("ipconfigs", ip).Info("cni add executed")

	err = k.store.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(PodBucket))
		if b == nil {
			return fmt.Errorf("bucket does not exist")
		}

		if err != nil {
			return err
		}

		js, err := json.Marshal(ip)

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
		cni.WithLabels(req.Metadata),
	}

	if req.Isolation == nil {
		return &beta.DetachNetworkResponse{}, nil
	}

	if req.Isolation.Path != "" {
		log.Infof("pod annotation id: %s netns: %s\n", req.Id, req.Isolation.Path)
	} else {
		log.Info("no network namespace path, this is either a bug or its running in the root")
		return &beta.DetachNetworkResponse{
		}, nil
	}

	err := k.c.Remove(ctx, req.Id, req.Isolation.Path, opts...)

	if err != nil {
		log.Errorf("unable to execute CNI DEL: %s",err.Error())

		return nil, err
	}

	err = k.store.Update(func (tx *bolt.Tx) error {
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

func (k *KniService) QueryPodNetwork(ctx context.Context,req *beta.QueryPodNetworkRequest) (*beta.QueryPodNetworkResponse, error) {
	
	log.Infof("query pod rpc request id: %s", req.Id)

	data := make(map[string]*beta.IPConfig)
	
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

	log.Infof("ipconfigs received for id: %s ip: %s", req.Id, data)

	if err != nil {
		return nil, err
	}

	return &beta.QueryPodNetworkResponse{
		Ipconfigs: data,
	}, nil
}

func (k *KniService) QueryNodeNetworks(ctx context.Context, req *beta.QueryNodeNetworksRequest) (*beta.QueryNodeNetworksResponse, error) {
	return nil, nil
}