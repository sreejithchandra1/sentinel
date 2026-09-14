package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/sentinel-monitoring/sentinel/internal/alerter"
	"github.com/sentinel-monitoring/sentinel/internal/api"
	"github.com/sentinel-monitoring/sentinel/internal/checker"
	"github.com/sentinel-monitoring/sentinel/internal/config"
	"github.com/sentinel-monitoring/sentinel/internal/scheduler"
	"github.com/sentinel-monitoring/sentinel/internal/store"
	"github.com/sentinel-monitoring/sentinel/internal/ui"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := store.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	smtpCfg, _ := db.GetSMTPConfig(cfg.SMTP)
	if smtpCfg.Host == "" {
		smtpCfg = cfg.SMTP
	}

	chk := checker.New(db)
	alt := alerter.New(db, smtpCfg, cfg.SMTP, cfg.Server.DashboardURL)
	sched := scheduler.New(db, chk, alt, cfg.Server.Workers, cfg.Server.RetentionDays)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sched.Start(ctx)

	apiSrv := api.New(db, alt, cfg)

	mux := http.NewServeMux()
	mux.Handle("/api/", apiSrv.Handler())
	mux.Handle("/api", apiSrv.Handler())
	mux.Handle("/", ui.Handler())

	server := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
	}

	go func() {
		log.Printf("sentinel listening on %s", cfg.Server.Listen)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down...")
	cancel()
	_ = server.Shutdown(context.Background())
}
