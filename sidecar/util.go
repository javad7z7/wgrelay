package main

import (
	"io"
	"net"
)

// relay copies data in both directions between two connections until either
// side closes, then tears both down.
func relay(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() { io.Copy(a, b); done <- struct{}{} }()
	go func() { io.Copy(b, a); done <- struct{}{} }()
	<-done
	a.Close()
	b.Close()
	<-done
}
