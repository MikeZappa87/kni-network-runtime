package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/MikeZappa87/kni-api/pkg/apis/runtime/beta"
	cniservice "github.com/mikezappa87/kni-network-runtime/pkg/cni-service"

	"google.golang.org/grpc"
)

func main() {
	var cmd, protocol, sockAddr string

	flag.StringVar(&cmd, "cmd", "cni", "backend")
	flag.StringVar(&protocol, "protocol", "unix", "protocol")
	flag.StringVar(&sockAddr, "address", "/tmp/kni.sock", "socket address")

	flag.Parse()

	if _, err := os.Stat(sockAddr); !os.IsNotExist(err) {
		if err := os.RemoveAll(sockAddr); err != nil {
			log.Fatal(err)
		}
	}

	listener, err := net.Listen(protocol, sockAddr)
	if err != nil {
		log.Fatal(err)
		return
	}

	server := grpc.NewServer()

	kni, err := cniservice.NewKniService()

	if err != nil {
		log.Fatal(err)
		return
	}

	beta.RegisterKNIServer(server, kni)

	fmt.Println("Running")

	server.Serve(listener)
}
