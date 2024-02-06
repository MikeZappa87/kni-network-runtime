package main

import (
	"flag"
	"os"

	libkni "github.com/MikeZappa87/libkni/pkg"
	libknicni "github.com/MikeZappa87/libkni/pkg/cni"
	log "github.com/sirupsen/logrus"
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

var protocol, sockAddr string

func main() {
	log.Info("network runtime started")

	flag.StringVar(&protocol, "protocol", "unix", "protocol")
	flag.StringVar(&sockAddr, "address", "/tmp/kni.sock", "socket address")

	flag.Parse()
	
	if err := libkni.NewDefaultKNIServer(sockAddr, protocol, libknicni.CreateDefaultConfig()); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
