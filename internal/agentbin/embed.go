package agentbin

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"strings"
)

//go:embed files/*
var binaries embed.FS

const minAgentBytes = 1024

func Binary(goos, arch string) ([]byte, error) {
	name, err := fileName(goos, arch)
	if err != nil {
		return nil, err
	}
	b, err := binaries.ReadFile("files/" + name)
	if err != nil {
		return nil, fmt.Errorf("agent binary not built")
	}
	if len(b) < minAgentBytes {
		return nil, fmt.Errorf("agent binary not built")
	}
	return b, nil
}

func SHA256(goos, arch string) (string, error) {
	b, err := Binary(goos, arch)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func fileName(goos, arch string) (string, error) {
	if strings.ToLower(strings.TrimSpace(goos)) != "linux" {
		return "", fmt.Errorf("unsupported os")
	}
	switch strings.ToLower(strings.TrimSpace(arch)) {
	case "amd64", "x86_64":
		return "linux-amd64", nil
	case "arm64", "aarch64":
		return "linux-arm64", nil
	default:
		return "", fmt.Errorf("unsupported arch")
	}
}
