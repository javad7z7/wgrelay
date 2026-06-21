package main

import (
	"context"
	"encoding/binary"
	"io"
	"log"
	"net"
	"strconv"
)

// socks5Server implements a minimal SOCKS5 proxy (CONNECT command, IPv4/IPv6/
// domain targets, optional username/password auth). Every outbound connection
// is dialed through the WireGuard tunnel.
type socks5Server struct {
	dial dialFunc
	user string
	pass string
}

func (s *socks5Server) serve(l net.Listener) {
	for {
		c, err := l.Accept()
		if err != nil {
			return
		}
		go s.handle(c)
	}
}

func (s *socks5Server) handle(c net.Conn) {
	defer c.Close()

	// --- Method negotiation ---
	head := make([]byte, 2)
	if _, err := io.ReadFull(c, head); err != nil || head[0] != 0x05 {
		return
	}
	methods := make([]byte, int(head[1]))
	if _, err := io.ReadFull(c, methods); err != nil {
		return
	}

	if s.user != "" {
		if !hasMethod(methods, 0x02) {
			c.Write([]byte{0x05, 0xFF})
			return
		}
		c.Write([]byte{0x05, 0x02})
		if !s.checkAuth(c) {
			return
		}
	} else {
		c.Write([]byte{0x05, 0x00})
	}

	// --- Request ---
	req := make([]byte, 4)
	if _, err := io.ReadFull(c, req); err != nil || req[0] != 0x05 {
		return
	}
	cmd, atyp := req[1], req[3]

	var host string
	switch atyp {
	case 0x01: // IPv4
		b := make([]byte, 4)
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		host = net.IP(b).String()
	case 0x03: // domain
		l := make([]byte, 1)
		if _, err := io.ReadFull(c, l); err != nil {
			return
		}
		d := make([]byte, int(l[0]))
		if _, err := io.ReadFull(c, d); err != nil {
			return
		}
		host = string(d)
	case 0x04: // IPv6
		b := make([]byte, 16)
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		host = net.IP(b).String()
	default:
		reply(c, 0x08) // address type not supported
		return
	}

	pb := make([]byte, 2)
	if _, err := io.ReadFull(c, pb); err != nil {
		return
	}
	port := binary.BigEndian.Uint16(pb)

	if cmd != 0x01 { // only CONNECT
		reply(c, 0x07)
		return
	}

	target := net.JoinHostPort(host, strconv.Itoa(int(port)))
	remote, err := s.dial(context.Background(), "tcp", target)
	if err != nil {
		log.Printf("socks5: dial %s: %v", target, err)
		reply(c, 0x05) // connection refused
		return
	}
	defer remote.Close()

	reply(c, 0x00) // success
	relay(c, remote)
}

func (s *socks5Server) checkAuth(c net.Conn) bool {
	h := make([]byte, 2)
	if _, err := io.ReadFull(c, h); err != nil || h[0] != 0x01 {
		return false
	}
	ub := make([]byte, int(h[1]))
	if _, err := io.ReadFull(c, ub); err != nil {
		return false
	}
	pl := make([]byte, 1)
	if _, err := io.ReadFull(c, pl); err != nil {
		return false
	}
	pb := make([]byte, int(pl[0]))
	if _, err := io.ReadFull(c, pb); err != nil {
		return false
	}
	if string(ub) == s.user && string(pb) == s.pass {
		c.Write([]byte{0x01, 0x00})
		return true
	}
	c.Write([]byte{0x01, 0x01})
	return false
}

func hasMethod(methods []byte, m byte) bool {
	for _, x := range methods {
		if x == m {
			return true
		}
	}
	return false
}

// reply sends a SOCKS5 reply with the given status and a null bind address.
func reply(c net.Conn, status byte) {
	c.Write([]byte{0x05, status, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
}
