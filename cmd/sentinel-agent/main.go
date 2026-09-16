package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

type agentConfig struct {
	ServerURL string `yaml:"server_url"`
	Token     string `yaml:"token"`
	HostID    string `yaml:"host_id"`
}

type ingestReply struct {
	OK     bool                   `json:"ok"`
	Config models.HostAgentConfig `json:"config"`
}

func main() {
	configPath := flag.String("config", "/etc/sentinel-agent/config.yaml", "path to agent config")
	diagnoseAuth := flag.Bool("diagnose-auth", false, "print auth log and journal access as this user, then exit")
	flag.Parse()

	if os.Geteuid() == 0 {
		log.Fatal("refusing to run as root; install as the sentinel-agent user")
	}
	if *diagnoseAuth {
		diagnoseAuthLogs()
		return
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if strings.TrimSpace(cfg.ServerURL) == "" || strings.TrimSpace(cfg.Token) == "" {
		log.Fatal("server_url and token are required in config (do not pass token on the command line)")
	}

	collect := models.HostAgentConfig{
		IntervalSeconds: models.DefaultHostIntervalSeconds,
		CollectCPU:      true,
		CollectMemory:   true,
		CollectDisk:     true,
		CollectLoad:     true,
		CollectSwap:     true,
		CollectIOWait:   true,
		CollectSecurity: true,
		CollectServices: true,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	client := &http.Client{Timeout: 15 * time.Second}
	interval := time.Duration(collect.IntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	run := func() {
		payload, err := collectMetrics(collect)
		if err != nil {
			log.Printf("collect: %v", err)
			return
		}
		payload.AgentVersion = models.HostAgentVersion
		next, err := ingest(client, cfg, payload)
		if err != nil {
			log.Printf("ingest: %v", err)
			return
		}
		collect = models.SanitizeAgentConfig(next)
		nextInterval := time.Duration(collect.IntervalSeconds) * time.Second
		if nextInterval != interval {
			interval = nextInterval
			ticker.Reset(interval)
		}
	}

	run()
	if len(collect.Services) > 0 {
		run()
	}
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			run()
		}
	}
}

func loadConfig(path string) (*agentConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg agentConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	cfg.ServerURL = strings.TrimSpace(cfg.ServerURL)
	cfg.Token = strings.TrimSpace(cfg.Token)
	return &cfg, nil
}

func ingest(client *http.Client, cfg *agentConfig, payload *models.HostIngestPayload) (models.HostAgentConfig, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return models.HostAgentConfig{}, err
	}
	url := strings.TrimRight(cfg.ServerURL, "/") + "/api/agent/ingest"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return models.HostAgentConfig{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return models.HostAgentConfig{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusUnauthorized {
			return models.HostAgentConfig{}, fmt.Errorf("status 401 (ingest token rejected; re-install so the service restarts with the new token)")
		}
		return models.HostAgentConfig{}, fmt.Errorf("status %d", resp.StatusCode)
	}
	var out ingestReply
	if err := json.Unmarshal(raw, &out); err != nil {
		return models.HostAgentConfig{}, err
	}
	return out.Config, nil
}
