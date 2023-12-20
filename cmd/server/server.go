package main

import (
	"flag"
	"net"
	"os"

	"github.com/MikeZappa87/kni-api/pkg/apis/runtime/beta"
	cniservice "github.com/mikezappa87/kni-network-runtime/pkg/cni-service"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func init() {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})
  
	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)
  
	// Only log the warning severity or above.
	log.SetLevel(log.DebugLevel)
  }

func main() {
	log.Info("network runtime started")

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

	kni, err := cniservice.NewKniService("eth", "net.db")

	if err != nil {
		log.Fatal(err)
		return
	}

	beta.RegisterKNIServer(server, kni)

	log.Info("kni network runtime is running")

	server.Serve(listener)
}
