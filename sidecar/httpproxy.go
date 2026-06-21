package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"log"
	"net"
	"net/http"
	"strings"
)

// httpProxy implements an HTTP proxy supporting both CONNECT (used by HTTPS,
// i.e. nearly all modern traffic) and plain HTTP forwarding. All outbound
// connections are dialed through the WireGuard tunnel.
type httpProxy struct {
	dial dialFunc
	user string
	pass string
}

func (h *httpProxy) serve(l net.Listener) {
	for {
		c, err := l.Accept()
		if err != nil {
			return
		}
		go h.handle(c)
	}
}

func (h *httpProxy) handle(c net.Conn) {
	br := bufio.NewReader(c)
	req, err := http.ReadRequest(br)
	if err != nil {
		c.Close()
		return
	}

	if h.user != "" && !h.checkAuth(req) {
		c.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\n" +
			"Proxy-Authenticate: Basic realm=\"wgrelay\"\r\n" +
			"Content-Length: 0\r\n\r\n"))
		c.Close()
		return
	}

	if req.Method == http.MethodConnect {
		h.handleConnect(c, req)
		return
	}
	h.handlePlain(c, req)
}

func (h *httpProxy) handleConnect(c net.Conn, req *http.Request) {
	defer c.Close()
	target := req.Host
	if !strings.Contains(target, ":") {
		target += ":443"
	}
	remote, err := h.dial(context.Background(), "tcp", target)
	if err != nil {
		log.Printf("http connect: dial %s: %v", target, err)
		c.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n"))
		return
	}
	defer remote.Close()
	c.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	relay(c, remote)
}

func (h *httpProxy) handlePlain(c net.Conn, req *http.Request) {
	defer c.Close()
	host := req.URL.Host
	if host == "" {
		host = req.Host
	}
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	remote, err := h.dial(context.Background(), "tcp", host)
	if err != nil {
		log.Printf("http: dial %s: %v", host, err)
		c.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n"))
		return
	}
	defer remote.Close()

	// Forward in origin form and drop proxy-only headers.
	req.RequestURI = ""
	req.Header.Del("Proxy-Connection")
	req.Header.Del("Proxy-Authorization")
	if err := req.Write(remote); err != nil {
		return
	}
	relay(c, remote)
}

func (h *httpProxy) checkAuth(req *http.Request) bool {
	const prefix = "Basic "
	auth := req.Header.Get("Proxy-Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return false
	}
	dec, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return false
	}
	u, p, ok := strings.Cut(string(dec), ":")
	return ok && u == h.user && p == h.pass
}
