package cniservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/MikeZappa87/kni-api/pkg/apis/runtime/beta"
	"github.com/containerd/go-cni"
	bolt "go.etcd.io/bbolt"
)

type KniService struct {
	c cni.CNI
	store *bolt.DB
}

const PodBucket = "pod"

func NewKniService() (beta.KNIServer, error) {
	opts := []cni.Opt{
		cni.WithInterfacePrefix("eth"),
		 cni.WithDefaultConf,
		 cni.WithLoNetwork} 
	
	cni, err := cni.New()

	if err != nil {
		return nil, err
	}
	
	db, err := bolt.Open("net.db", 0600, nil)
	if err != nil {
  		return nil, err
	}

	kni := &KniService{
		c: cni,
		store: db,
	}

	db.Update(func(tx *bolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte(PodBucket))

		return nil
	})

	err = kni.c.Load(opts...)

	if err != nil {
		return nil, err
	}

	return kni, nil
}

func (k *KniService) AttachNetwork(ctx context.Context, req *beta.AttachNetworkRequest) (*beta.AttachNetworkResponse, error) {
	//This is nice. In the container runtime if you want to add one you need to contribute this to the container runtime
	//This way you are in complete control. Better capability support with KNI -> CNI
	
	opts := []cni.NamespaceOpts{
		cni.WithArgs("IgnoreUnknown", "1"),
		cni.WithLabels(req.Labels),
		cni.WithLabels(req.Annotations),
		cni.WithLabels(req.Metadata),
	}

	if val, ok := req.Annotations["netns"]; ok {
		fmt.Printf("annotations netns: %s\n", val)
	}
	
	fmt.Printf("id: %s netns: %s\n", req.Id, req.Isolation.Path)

	res, err := k.c.SetupSerially(ctx, req.Id, req.Isolation.Path, opts...)

	fmt.Println("ATTACH RECEIVED")

	if err != nil {
		fmt.Println(fmt.Errorf("issue attaching with cni: %s",err.Error()))

		return nil, err
	}

	ip := make(map[string]*beta.IPConfig)

	for outk, outv := range res.Interfaces {
		data := &beta.IPConfig{}
		ip[outk] = data
		data.Mac = outv.Mac

		for _, v := range outv.IPConfigs {
			data.Ip = append(data.Ip, v.IP.String())
			fmt.Printf("interface: %s ip: %s\n", outk, v.IP.String())
		}
	}

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
		fmt.Println(fmt.Errorf("issue attaching with json marshalling: %s",err.Error()))
		return nil, err
	}

	return &beta.AttachNetworkResponse{
		Ipconfigs: ip,
	}, nil
}

func (k *KniService) DetachNetwork(ctx context.Context, req *beta.DetachNetworkRequest) (*beta.DetachNetworkResponse, error) {
	
	opts := []cni.NamespaceOpts{
		cni.WithArgs("IgnoreUnknown", "1"),
		cni.WithLabels(req.Labels),
		cni.WithLabels(req.Annotations),
		cni.WithLabels(req.Metadata),
	}

	if req.Isolation == nil {
		return &beta.DetachNetworkResponse{}, nil
	}

	fmt.Printf("id: %s netns: %s\n", req.Id, req.Isolation.Path)
	
	err := k.c.Remove(ctx, req.Id, req.Isolation.Path, opts...)

	if err != nil {
		fmt.Println(fmt.Errorf("issue detaching with cni: %s",err.Error()))

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
		return nil, err
	}

	fmt.Println("DETACH RECEIVED")

	return &beta.DetachNetworkResponse{}, nil
}

func (k *KniService) SetupNodeNetwork(context.Context, *beta.SetupNodeNetworkRequest) (*beta.SetupNodeNetworkResponse, error) {
	//Setup the initial node network

	return nil, nil
}

func (k *KniService) QueryPodNetwork(ctx context.Context,req *beta.QueryPodNetworkRequest) (*beta.QueryPodNetworkResponse, error) {
	
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