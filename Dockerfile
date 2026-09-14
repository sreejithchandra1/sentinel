FROM node:20-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.25.9-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=web /app/internal/ui/dist ./internal/ui/dist
RUN CGO_ENABLED=1 go build -o sentinel ./cmd/sentinel

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -H sentinel
WORKDIR /app
COPY --from=builder /app/sentinel /usr/local/bin/sentinel
COPY config.example.yaml /etc/sentinel/config.yaml
RUN mkdir -p /var/lib/sentinel && chown sentinel:sentinel /var/lib/sentinel
USER sentinel
EXPOSE 8082
ENTRYPOINT ["sentinel", "-config", "/etc/sentinel/config.yaml"]
