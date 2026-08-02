// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	directory := flag.String("dir", "dist/docs-portal", "generated portal directory")
	address := flag.String("address", "127.0.0.1:8080", "localhost listen address")
	smoke := flag.Bool("smoke", false, "serve on a temporary local port, render the portal, then exit")
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
	listenAddress := *address
	if *smoke {
		listenAddress = net.JoinHostPort(host, "0")
	}
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	server := &http.Server{Handler: http.FileServer(http.Dir(*directory)), ReadHeaderTimeout: 5 * time.Second}
	url := "http://" + listener.Addr().String()
	fmt.Println(url)
	if *smoke {
		if err := smokeTest(server, listener, url); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func smokeTest(server *http.Server, listener net.Listener, baseURL string) error {
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get(baseURL + "/index.html")
	if err != nil {
		return fmt.Errorf("render documentation portal: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "<h1>HubSpot configuration surfaces</h1>") {
		return errors.New("documentation portal smoke render returned unexpected content")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return err
	}
	if err := <-serveResult; err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
