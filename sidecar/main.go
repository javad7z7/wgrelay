package main

import (
	"flag"
	"log"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	configPath := flag.String("config", "", "path to WireGuard .conf file (required)")
	socksAddr := flag.String("socks", "127.0.0.1:1080", "SOCKS5 listen address (empty to disable)")
	httpAddr := flag.String("http", "127.0.0.1:8080", "HTTP proxy listen address (empty to disable)")
	user := flag.String("user", "", "optional proxy username (applies to both proxies)")
	pass := flag.String("pass", "", "optional proxy password")
	dnsFlag := flag.String("dns", "", "override DNS server(s), comma-separated")
	verbose := flag.Bool("verbose", false, "verbose WireGuard logging")
	flag.Parse()

	if *configPath == "" {
		log.Fatal("missing -config (path to a WireGuard .conf file)")
	}

	cfg, err := parseConfig(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	var dnsOverride []netip.Addr
	if *dnsFlag != "" {
		for _, d := range splitList(*dnsFlag) {
			ip, err := netip.ParseAddr(d)
			if err != nil {
				log.Fatalf("invalid -dns %q: %v", d, err)
			}
			dnsOverride = append(dnsOverride, ip)
		}
	}

	if *socksAddr == "" && *httpAddr == "" {
		log.Fatal("both -socks and -http are disabled; nothing to do")
	}

	t, err := startTunnel(cfg, dnsOverride, *verbose)
	if err != nil {
		log.Fatalf("tunnel: %v", err)
	}
	defer t.Close()
	log.Print("WireGuard tunnel up (userspace, no routing changes)")

	if *socksAddr != "" {
		l, err := net.Listen("tcp", *socksAddr)
		if err != nil {
			log.Fatalf("socks listen: %v", err)
		}
		go (&socks5Server{dial: t.dial, user: *user, pass: *pass}).serve(l)
		log.Printf("SOCKS5 proxy listening on %s", *socksAddr)
	}

	if *httpAddr != "" {
		l, err := net.Listen("tcp", *httpAddr)
		if err != nil {
			log.Fatalf("http listen: %v", err)
		}
		go (&httpProxy{dial: t.dial, user: *user, pass: *pass}).serve(l)
		log.Printf("HTTP proxy listening on %s", *httpAddr)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Print("shutting down")
}
