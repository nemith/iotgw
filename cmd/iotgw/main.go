package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	"github.com/nemith/iotgw"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("you need to specify at least one interface")
	}

	intfs := make([]*net.Interface, 0, len(os.Args[1:]))
	for _, arg := range os.Args[1:] {
		intf, err := net.InterfaceByName(arg)
		log.Printf("Adding interface %s (MTU: %d)", intf.Name, intf.MTU)
		if err != nil {
			log.Fatalf("failed to find interface '%s': %v", arg, err)
		}
		intfs = append(intfs, intf)
	}

	proxy := iotgw.NewMDNSProxy(intfs, nil)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(5 * time.Second)
		cancel()
	}()

	if err := proxy.Listen(ctx); err != nil {
		log.Fatalf("failed to listen: %v", err)

	}

}
