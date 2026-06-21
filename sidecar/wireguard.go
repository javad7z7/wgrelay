package main

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

// dialFunc dials a TCP connection through the WireGuard tunnel.
type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

type tunnel struct {
	dev  *device.Device
	dial dialFunc
}

// startTunnel brings up a fully userspace WireGuard interface backed by a
// gVisor netstack. No utun device is created and the system routing table is
// never touched, so this needs no root and leaves all other traffic alone.
func startTunnel(cfg *wgConfig, dnsOverride []netip.Addr, verbose bool) (*tunnel, error) {
	dns := cfg.dns
	if len(dnsOverride) > 0 {
		dns = dnsOverride
	}
	if len(dns) == 0 {
		// Domain-name proxy requests need a resolver reachable through the
		// tunnel; fall back to a public one if the config didn't set DNS.
		dns = []netip.Addr{netip.MustParseAddr("1.1.1.1")}
	}

	tunDev, tnet, err := netstack.CreateNetTUN(cfg.addresses, dns, cfg.mtu)
	if err != nil {
		return nil, fmt.Errorf("create netstack tun: %w", err)
	}

	level := device.LogLevelError
	if verbose {
		level = device.LogLevelVerbose
	}
	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), device.NewLogger(level, "wg "))

	uapi, err := cfg.uapi()
	if err != nil {
		return nil, err
	}
	if err := dev.IpcSet(uapi); err != nil {
		return nil, fmt.Errorf("configure device: %w", err)
	}
	if err := dev.Up(); err != nil {
		return nil, fmt.Errorf("bring device up: %w", err)
	}

	return &tunnel{dev: dev, dial: tnet.DialContext}, nil
}

func (t *tunnel) Close() {
	if t.dev != nil {
		t.dev.Close()
	}
}
