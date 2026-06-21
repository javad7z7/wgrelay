package main

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

type peerConfig struct {
	publicKey    string
	presharedKey string
	endpoint     string
	allowedIPs   []string
	keepalive    int
}

type wgConfig struct {
	privateKey string
	addresses  []netip.Addr // [Interface] Address (CIDR stripped)
	dns        []netip.Addr // [Interface] DNS
	mtu        int
	peers      []peerConfig
}

// parseConfig reads a standard WireGuard .conf (the same INI format the
// official apps use) and returns a parsed configuration.
func parseConfig(path string) (*wgConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &wgConfig{mtu: 1420}
	var section string
	var cur *peerConfig

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			if section == "peer" {
				cfg.peers = append(cfg.peers, peerConfig{})
				cur = &cfg.peers[len(cfg.peers)-1]
			}
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])

		switch section {
		case "interface":
			switch key {
			case "privatekey":
				cfg.privateKey = val
			case "address":
				for _, a := range splitList(val) {
					ip, err := stripCIDR(a)
					if err != nil {
						return nil, fmt.Errorf("invalid Address %q: %w", a, err)
					}
					cfg.addresses = append(cfg.addresses, ip)
				}
			case "dns":
				for _, d := range splitList(val) {
					if ip, err := netip.ParseAddr(strings.TrimSpace(d)); err == nil {
						cfg.dns = append(cfg.dns, ip)
					}
				}
			case "mtu":
				if n, err := strconv.Atoi(val); err == nil {
					cfg.mtu = n
				}
			}
		case "peer":
			switch key {
			case "publickey":
				cur.publicKey = val
			case "presharedkey":
				cur.presharedKey = val
			case "endpoint":
				cur.endpoint = val
			case "allowedips":
				cur.allowedIPs = append(cur.allowedIPs, splitList(val)...)
			case "persistentkeepalive":
				if n, err := strconv.Atoi(val); err == nil {
					cur.keepalive = n
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if cfg.privateKey == "" {
		return nil, fmt.Errorf("missing PrivateKey in [Interface]")
	}
	if len(cfg.peers) == 0 {
		return nil, fmt.Errorf("no [Peer] section found")
	}
	return cfg, nil
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func stripCIDR(s string) (netip.Addr, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "/") {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return netip.Addr{}, err
		}
		return p.Addr(), nil
	}
	return netip.ParseAddr(s)
}

// uapi renders the configuration into the line-based format expected by
// wireguard-go's IpcSet. Keys are converted from base64 to hex, and the
// endpoint hostname is resolved to an IP:port.
func (c *wgConfig) uapi() (string, error) {
	var b strings.Builder

	pk, err := keyToHex(c.privateKey)
	if err != nil {
		return "", fmt.Errorf("PrivateKey: %w", err)
	}
	fmt.Fprintf(&b, "private_key=%s\n", pk)

	for _, p := range c.peers {
		pub, err := keyToHex(p.publicKey)
		if err != nil {
			return "", fmt.Errorf("peer PublicKey: %w", err)
		}
		fmt.Fprintf(&b, "public_key=%s\n", pub)

		if p.presharedKey != "" {
			psk, err := keyToHex(p.presharedKey)
			if err != nil {
				return "", fmt.Errorf("peer PresharedKey: %w", err)
			}
			fmt.Fprintf(&b, "preshared_key=%s\n", psk)
		}
		if p.endpoint != "" {
			ep, err := resolveEndpoint(p.endpoint)
			if err != nil {
				return "", fmt.Errorf("peer Endpoint %q: %w", p.endpoint, err)
			}
			fmt.Fprintf(&b, "endpoint=%s\n", ep)
		}
		if p.keepalive > 0 {
			fmt.Fprintf(&b, "persistent_keepalive_interval=%d\n", p.keepalive)
		}
		for _, a := range p.allowedIPs {
			fmt.Fprintf(&b, "allowed_ip=%s\n", a)
		}
	}
	return b.String(), nil
}

func keyToHex(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return "", err
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("expected 32-byte key, got %d bytes", len(raw))
	}
	return hex.EncodeToString(raw), nil
}

// resolveEndpoint turns "host:port" into "ip:port". DNS resolution here uses
// the OS resolver (outside the tunnel), which is correct for the handshake.
func resolveEndpoint(ep string) (string, error) {
	host, port, err := net.SplitHostPort(ep)
	if err != nil {
		return "", err
	}
	if net.ParseIP(host) != nil {
		return ep, nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}
	var chosen net.IP
	for _, ip := range ips { // prefer IPv4
		if ip.To4() != nil {
			chosen = ip
			break
		}
	}
	if chosen == nil && len(ips) > 0 {
		chosen = ips[0]
	}
	if chosen == nil {
		return "", fmt.Errorf("no addresses for %s", host)
	}
	return net.JoinHostPort(chosen.String(), port), nil
}
