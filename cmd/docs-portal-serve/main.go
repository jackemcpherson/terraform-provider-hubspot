// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	directory := flag.String("dir", "dist/docs-portal", "generated portal directory")
	address := flag.String("address", "127.0.0.1:8080", "localhost listen address")
	flag.Parse()
	host, _, err := net.SplitHostPort(*address)
	if err != nil || (host != "localhost" && (net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback())) {
		fmt.Fprintln(os.Stderr, "documentation portal address must use a loopback host")
		os.Exit(1)
	}
	if _, err := os.Stat(*directory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	server := &http.Server{Addr: *address, Handler: http.FileServer(http.Dir(*directory)), ReadHeaderTimeout: 5 * time.Second}
	fmt.Printf("http://%s\n", *address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
