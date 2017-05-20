package main

import (
	"github.com/drausin/libri/libri/cmd"
	"net"
	"os"
	"log"
)

func main() {
	addr, err := net.ResolveTCPAddr("tcp4", "librarians-0.libri.default.svc.cluster.local:20100")
	if err != nil {
		panic(err)
	}
	log.Printf("%s -> %v", os.Args[1], addr)
	cmd.Execute()
}
