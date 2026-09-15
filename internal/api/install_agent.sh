#!/bin/sh
# Sentinel host agent installer. Requires root for install only.
# The running service uses the unprivileged sentinel-agent user.
set -eu

SENTINEL_URL="__SENTINEL_URL__"
TOKEN="__ENROLL_TOKEN__"
AGENT_USER="sentinel-agent"
BIN_PATH="/usr/local/bin/sentinel-agent"
CONF_DIR="/etc/sentinel-agent"
CONF_FILE="${CONF_DIR}/config.yaml"
UNIT_PATH="/etc/systemd/system/sentinel-agent.service"

if [ "$(id -u)" -ne 0 ]; then
  echo "this installer must be run as root (sudo)" >&2
  exit 1
fi

if [ ! -d /run/systemd/system ]; then
  echo "systemd is required" >&2
  exit 1
fi

arch="$(uname -m)"
case "$arch" in
  x86_64) arch="amd64" ;;
  aarch64) arch="arm64" ;;
  amd64|arm64) ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "downloading agent..."
curl -fsSL "${SENTINEL_URL}/api/hosts/agent/linux/${arch}?token=${TOKEN}" -o "${tmp}/sentinel-agent"
chmod 0755 "${tmp}/sentinel-agent"

want="$(curl -fsSL "${SENTINEL_URL}/api/hosts/agent/linux/${arch}/sha256?token=${TOKEN}" | tr -d '[:space:]')"
got="$(sha256sum "${tmp}/sentinel-agent" | awk '{print $1}')"
if [ -z "$want" ] || [ "$want" != "$got" ]; then
  echo "agent checksum mismatch" >&2
  exit 1
fi

hostname_val="$(hostname -f 2>/dev/null || hostname 2>/dev/null || echo unknown)"
enroll_json="$(printf '{"token":"%s","hostname":"%s","os":"linux","arch":"%s"}' "$TOKEN" "$hostname_val" "$arch")"
enroll_out="$(curl -fsSL -X POST -H 'Content-Type: application/json' \
  -d "$enroll_json" "${SENTINEL_URL}/api/agent/enroll")"

ingest_token="$(printf '%s' "$enroll_out" | sed -n 's/.*"ingest_token":"\([^"]*\)".*/\1/p')"
host_id="$(printf '%s' "$enroll_out" | sed -n 's/.*"host_id":"\([^"]*\)".*/\1/p')"
interval="$(printf '%s' "$enroll_out" | sed -n 's/.*"interval_seconds":\([0-9]*\).*/\1/p')"
if [ -z "$ingest_token" ] || [ -z "$host_id" ]; then
  echo "enrollment failed" >&2
  exit 1
fi
[ -n "$interval" ] || interval=30

if ! id -u "$AGENT_USER" >/dev/null 2>&1; then
  if command -v useradd >/dev/null 2>&1; then
    useradd --system --no-create-home --shell /usr/sbin/nologin "$AGENT_USER"
  else
    adduser --system --no-create-home --group --shell /usr/sbin/nologin "$AGENT_USER"
  fi
fi

extra_groups=""
if getent group adm >/dev/null 2>&1; then extra_groups="${extra_groups} adm"; fi
if getent group systemd-journal >/dev/null 2>&1; then extra_groups="${extra_groups} systemd-journal"; fi
if [ -n "$extra_groups" ]; then
  usermod -aG $extra_groups "$AGENT_USER" >/dev/null 2>&1 || true
fi

install -o root -g root -m 0755 "${tmp}/sentinel-agent" "$BIN_PATH"
install -d -o root -g "$AGENT_USER" -m 0750 "$CONF_DIR"
umask 077
cat > "$CONF_FILE" <<EOF
server_url: ${SENTINEL_URL}
token: ${ingest_token}
host_id: ${host_id}
EOF
chown root:"$AGENT_USER" "$CONF_FILE"
chmod 0640 "$CONF_FILE"

supp_line=""
if [ -n "$extra_groups" ]; then
  supp_line="SupplementaryGroups=${extra_groups}"
fi

cat > "$UNIT_PATH" <<EOF
[Unit]
Description=Sentinel host agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${AGENT_USER}
Group=${AGENT_USER}
${supp_line}
ExecStart=${BIN_PATH} -config ${CONF_FILE}
Restart=on-failure
RestartSec=10
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictNamespaces=true
RestrictRealtime=true
LockPersonality=true
MemoryDenyWriteExecute=true
CapabilityBoundingSet=
AmbientCapabilities=
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
SystemCallArchitectures=native

[Install]
WantedBy=multi-user.target
EOF
chmod 0644 "$UNIT_PATH"

systemctl daemon-reload
systemctl enable --now sentinel-agent.service
echo "sentinel-agent installed and started as ${AGENT_USER}"
