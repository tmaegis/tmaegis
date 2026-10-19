// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

// Command tmaegis-server runs the self-hosted TMAegis API and dashboard.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tmaegis/tmaegis/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	rulesDir := flag.String("rules-dir", "./rules", "directory containing rule YAML files")
	dataDir := flag.String("data-dir", "./data", "directory for stored scan reports")
	flag.Parse()

	token := os.Getenv("TMAEGIS_SERVER_TOKEN")
	if token == "" {
		log.Println("warning: TMAEGIS_SERVER_TOKEN is not set; report endpoints are disabled")
	}
	srv, err := server.New(server.Config{RulesDir: *rulesDir, DataDir: *dataDir, Token: token})
	if err != nil {
		log.Fatal(err)
	}
	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		httpSrv.Shutdown(ctx)
	}()
	log.Printf("tmaegis-server listening on %s", *addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
