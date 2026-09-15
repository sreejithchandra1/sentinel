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

nologin_shell="/usr/sbin/nologin"
[ -x /sbin/nologin ] && nologin_shell="/sbin/nologin"
if ! id -u "$AGENT_USER" >/dev/null 2>&1; then
  if command -v useradd >/dev/null 2>&1; then
    useradd --system --no-create-home --shell "$nologin_shell" "$AGENT_USER"
  else
    adduser --system --no-create-home --group --shell "$nologin_shell" "$AGENT_USER"
  fi
fi

extra_groups=""
add_grp() {
  if getent group "$1" >/dev/null 2>&1; then
    if [ -z "$extra_groups" ]; then
      extra_groups="$1"
    else
      extra_groups="$extra_groups $1"
    fi
  fi
}
add_grp adm
add_grp systemd-journal
if [ -n "$extra_groups" ]; then
  grp_csv="$(echo "$extra_groups" | tr ' ' ',')"
  usermod -aG "$grp_csv" "$AGENT_USER" || echo "warning: usermod -aG ${grp_csv} failed" >&2
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

# Shared sandbox: keep diagnose oneshot identical to the running agent.
# Do not bind-mount /var/log/secure read-only — that makes setfacl fail with
# "Read-only file system". ACL is applied by sentinel-agent-acl.service instead.
unit_sandbox() {
  cat <<EOF
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
BindReadOnlyPaths=-/var/log/journal -/run/log/journal -/run/systemd/journal
EOF
}

cat > "$UNIT_PATH" <<EOF
[Unit]
Description=Sentinel host agent
After=network-online.target sentinel-agent-acl.service
Wants=network-online.target sentinel-agent-acl.service

[Service]
Type=simple
User=${AGENT_USER}
Group=${AGENT_USER}
${supp_line}
ExecStart=${BIN_PATH} -config ${CONF_FILE}
Restart=on-failure
RestartSec=10
$(unit_sandbox)

[Install]
WantedBy=multi-user.target
EOF
chmod 0644 "$UNIT_PATH"

DIAG_PATH="/etc/systemd/system/sentinel-agent-diagnose.service"
cat > "$DIAG_PATH" <<EOF
[Unit]
Description=Sentinel host agent auth-log diagnose
After=network-online.target sentinel-agent-acl.service
Wants=sentinel-agent-acl.service

[Service]
Type=oneshot
User=${AGENT_USER}
Group=${AGENT_USER}
${supp_line}
ExecStart=${BIN_PATH} -diagnose-auth
$(unit_sandbox)
EOF
chmod 0644 "$DIAG_PATH"

ACL_PATH="/etc/systemd/system/sentinel-agent-acl.service"
cat > "$ACL_PATH" <<EOF
[Unit]
Description=ACL so sentinel-agent can read auth logs
Before=sentinel-agent.service sentinel-agent-diagnose.service

[Service]
Type=oneshot
ExecStart=-/usr/bin/setfacl -m u:${AGENT_USER}:r /var/log/secure
ExecStart=-/usr/bin/setfacl -m u:${AGENT_USER}:r /var/log/auth.log
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF
chmod 0644 "$ACL_PATH"

if [ -d /etc/cron.hourly ]; then
  cat > /etc/cron.hourly/sentinel-agent-acl <<EOF
#!/bin/sh
# Re-apply after logrotate recreates 0600 auth logs.
[ -f /var/log/secure ] && /usr/bin/setfacl -m u:${AGENT_USER}:r /var/log/secure 2>/dev/null || true
[ -f /var/log/auth.log ] && /usr/bin/setfacl -m u:${AGENT_USER}:r /var/log/auth.log 2>/dev/null || true
EOF
  chmod 0755 /etc/cron.hourly/sentinel-agent-acl
fi

systemctl daemon-reload
systemctl enable sentinel-agent-acl.service
systemctl start sentinel-agent-acl.service || true
systemctl enable sentinel-agent.service
# enable --now does not restart an already-running unit, so a re-install would
# write a new ingest token and leave the old process 401ing.
systemctl restart sentinel-agent.service
echo "sentinel-agent installed and started as ${AGENT_USER}"
echo "to test auth logs as ${AGENT_USER}:"
echo "  sudo -u ${AGENT_USER} ${BIN_PATH} -diagnose-auth"
echo "or (same systemd sandbox as the service):"
echo "  systemctl start sentinel-agent-diagnose && journalctl -u sentinel-agent-diagnose -e --no-pager"
