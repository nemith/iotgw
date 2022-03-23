package iotgw

import (
	"context"
	"log"
	"net"
	"strconv"
	"sync"
)

const mdnsPort = 5353

var (
	mdnsIPv4Addr = net.IPv4(224, 0, 0, 251) // 224.0.0.251
	mdnsUDPAddr4 = &net.UDPAddr{IP: mdnsIPv4Addr, Port: mdnsPort}

	mdnsIPv6Addr = net.IP{0xff, 0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xfb} // ff02::fb
	mdnsUDPAddr6 = &net.UDPAddr{IP: mdnsIPv6Addr, Port: mdnsPort}
)

type MDNSProxyConfig struct {
	DisableIPv4 bool
	DisableIPv6 bool
}

type MDNSProxy struct {
	intfs []*net.Interface
	cfg   *MDNSProxyConfig
}

func NewMDNSProxy(interfaces []*net.Interface, config *MDNSProxyConfig) *MDNSProxy {
	if config == nil {
		config = &MDNSProxyConfig{}
	}

	return &MDNSProxy{
		intfs: interfaces,
		cfg:   config,
	}
}

func (m *MDNSProxy) Listen(ctx context.Context) error {
	wg := sync.WaitGroup{}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if !m.cfg.DisableIPv4 {
		conn4, err := newConn4(ctx, ":"+strconv.Itoa(mdnsPort))
		if err != nil {
			return err
		}
		defer conn4.Close()

		wg.Add(1)
		go func() {
			m.listen(conn4, mdnsUDPAddr4)
			wg.Done()
		}()

	}

	if !m.cfg.DisableIPv6 {
		conn6, err := newConn6(ctx, ":"+strconv.Itoa(mdnsPort))
		if err != nil {
			return err
		}
		defer conn6.Close()

		wg.Add(1)
		go func() {
			m.listen(conn6, mdnsUDPAddr6)
			wg.Done()
		}()
	}

	wg.Wait()
	return nil
}

const maxPktSize = 9000 // https://datatracker.ietf.org/doc/html/rfc6762#section-17

func (m *MDNSProxy) listen(conn conn, groupAddr net.Addr) error {
	buf := make([]byte, maxPktSize)

	for _, ifi := range m.intfs {
		log.Printf("joining group %s on %s", groupAddr, ifi.Name)
		if err := conn.JoinGroup(ifi, groupAddr); err != nil {
			return err
		}
	}

	for {
		n, cm, src, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf("failed to read from conn: %v", err)
			if nerr, ok := err.(net.Error); ok {
				if !nerr.Temporary() {
					return err
				}
			}
			continue
		}
		log.Printf("received packet on %d from %s (%d bytes)", cm.IfIndex, src, n)

		for _, intf := range m.intfs {
			if intf.Index == cm.IfIndex {
				// skip writing message back out the same interface
				continue
			}

			// set the outbound interface
			outCM := controlMsg{
				IfIndex: intf.Index,
			}

			n, err := conn.WriteTo(buf[:n], &outCM, groupAddr)
			if err != nil {
				log.Printf("failed to write message to %s%%%s: %v", groupAddr, intf.Name, err)
				continue
			}
			log.Printf("wrote %d bytes to %s", n, intf.Name)
		}
	}
}
