.PHONY: web agents build run clean install test

AGENT_DIR=internal/agentbin/files

web:
	cd web && npm install && npm run build

agents:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(AGENT_DIR)/linux-amd64 ./cmd/sentinel-agent
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(AGENT_DIR)/linux-arm64 ./cmd/sentinel-agent

build: web agents
	go build -o bin/sentinel ./cmd/sentinel

test:
	go test ./...

run: build
	./bin/sentinel -config config.example.yaml

clean:
	rm -rf bin internal/ui/dist/* web/node_modules web/dist
