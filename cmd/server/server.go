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

var cmd, protocol, sockAddr, ifprefix, dbname string

const DEFAULT_IFPREFIX = "eth"
const DEFAULT_DBNAME = "net.db"

func main() {
	log.Info("network runtime started")

	flag.StringVar(&cmd, "cmd", "cni", "backend")
	flag.StringVar(&protocol, "protocol", "unix", "protocol")
	flag.StringVar(&sockAddr, "address", "/tmp/kni.sock", "socket address")
	flag.StringVar(&ifprefix, "ifprefix", "eth", "interface prefix")
	flag.StringVar(&dbname, "dbname", "net.db", "boltdb file name")

	flag.Parse()

	if err := run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func run() error {

	if ifprefix == "" {
		ifprefix = DEFAULT_IFPREFIX
	}

	if dbname == "" {
		dbname = DEFAULT_DBNAME
	}

	if _, err := os.Stat(sockAddr); !os.IsNotExist(err) {
		if err := os.RemoveAll(sockAddr); err != nil {
			log.Fatal(err)
		}
	}

	listener, err := net.Listen(protocol, sockAddr)
	if err != nil {
		log.Fatal(err)
		return err
	}

	server := grpc.NewServer()

	kni, err := cniservice.NewKniService(ifprefix, dbname)

	if err != nil {
		log.Fatal(err)
		return err
	}

	beta.RegisterKNIServer(server, kni)

	log.Info("kni network runtime is running")

	return server.Serve(listener)
}
