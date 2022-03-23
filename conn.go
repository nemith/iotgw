package iotgw

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"
)

// controlMsg is a protocol independant version of ipv4.ControlMessage or
// ipv6.ControlMessage
type controlMsg struct {
	HopLimit int // TTL on ipv4
	IfIndex  int
	Dst      net.IP
}

// conn is a protocol indepdenant interface for ipv4.PacketConn or
// ipv6.PacketConn
type conn interface {
	ReadFrom([]byte) (int, *controlMsg, net.Addr, error)
	WriteTo([]byte, *controlMsg, net.Addr) (int, error)
	JoinGroup(*net.Interface, net.Addr) error
}

func setReuseAddress(c syscall.RawConn) (err error) {
	c.Control(func(fd uintptr) {
		// Set SO_REUSEADDR
		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
		if err != nil {
			return
		}

		// Set SO_REUSEPORT
		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		if err != nil {
			return
		}
	})

	return err
}

type conn4 struct {
	*ipv4.PacketConn
}

func (c *conn4) ReadFrom(p []byte) (int, *controlMsg, net.Addr, error) {
	n, v4cm, src, err := c.PacketConn.ReadFrom(p)
	cm := controlMsg{
		HopLimit: v4cm.TTL,
		IfIndex:  v4cm.IfIndex,
		Dst:      v4cm.Dst,
	}

	return n, &cm, src, err
}

func (c *conn4) WriteTo(p []byte, cm *controlMsg, dst net.Addr) (int, error) {
	v4cm := ipv4.ControlMessage{
		TTL:     cm.HopLimit,
		IfIndex: cm.IfIndex,
		Dst:     cm.Dst,
	}
	return c.PacketConn.WriteTo(p, &v4cm, dst)
}

func newConn4(ctx context.Context, addr string) (*conn4, error) {
	lnConfig := net.ListenConfig{
		Control: func(_ string, _ string, c syscall.RawConn) error {
			return setReuseAddress(c)
		},
	}

	netConn, err := lnConfig.ListenPacket(ctx, "udp4", addr)
	if err != nil {
		return nil, err
	}
	conn := ipv4.NewPacketConn(netConn)

	// Enable mutlicast loop to see packets from other local programs
	if err := conn.SetMulticastLoopback(true); err != nil {
		return nil, err
	}

	// TTL should be 255 according to RFC....
	if err := conn.SetMulticastTTL(255); err != nil {
		return nil, err
	}

	// Get the destination address, ttl, and incoming interface for security
	// and split horizon checks.
	if err := conn.SetControlMessage(ipv4.FlagDst|ipv4.FlagInterface|ipv4.FlagTTL, true); err != nil {
		return nil, err
	}

	return &conn4{conn}, nil
}

type conn6 struct {
	*ipv6.PacketConn
}

func (c *conn6) ReadFrom(p []byte) (int, *controlMsg, net.Addr, error) {
	n, v6cm, src, err := c.PacketConn.ReadFrom(p)
	cm := controlMsg{
		HopLimit: v6cm.HopLimit,
		IfIndex:  v6cm.IfIndex,
		Dst:      v6cm.Dst,
	}

	return n, &cm, src, err
}

func (c *conn6) WriteTo(p []byte, cm *controlMsg, dst net.Addr) (int, error) {
	v6cm := ipv6.ControlMessage{
		HopLimit: cm.HopLimit,
		IfIndex:  cm.IfIndex,
		Dst:      cm.Dst,
	}
	return c.PacketConn.WriteTo(p, &v6cm, dst)
}

func newConn6(ctx context.Context, addr string) (*conn6, error) {
	lnConfig := net.ListenConfig{
		Control: func(_ string, _ string, c syscall.RawConn) error {
			return setReuseAddress(c)
		},
	}

	netConn, err := lnConfig.ListenPacket(ctx, "udp6", addr)
	if err != nil {
		return nil, err
	}
	conn := ipv6.NewPacketConn(netConn)

	// Enable mutlicast loop to see packets from other local programs
	if err := conn.SetMulticastLoopback(true); err != nil {
		return nil, err
	}

	// TTL should be 255 according to RFC....
	if err := conn.SetMulticastHopLimit(255); err != nil {
		return nil, err
	}

	// Get the destination address, ttl, and incoming interface for security
	// and split horizon checks.
	if err := conn.SetControlMessage(ipv6.FlagDst|ipv6.FlagInterface|ipv6.FlagHopLimit, true); err != nil {
		return nil, err
	}

	return &conn6{conn}, nil
}
