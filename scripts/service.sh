#!/usr/bin/env bash
set -euo pipefail

command_name="${1:-status}"
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
service_name="${ROUTER_SERVICE_NAME:-yeying-router-localhost}"
binary_path="${ROUTER_BINARY:-$root_dir/build/router}"
port="${ROUTER_PORT:-3011}"
log_dir="${ROUTER_LOG_DIR:-$root_dir/logs}"

die() {
  echo "Error: $*" >&2
  exit 1
}

require_binary() {
  [[ -x "$binary_path" ]] || die "Router binary not found or not executable: $binary_path. Build with: go build -o build/router ./cmd/router"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "Required command not found: $1"
}

require_safe_service_name() {
  [[ "$service_name" =~ ^[A-Za-z0-9._-]+$ ]] || die "ROUTER_SERVICE_NAME contains unsupported characters: $service_name"
}

run_privileged() {
  if [[ "$(id -u)" -eq 0 ]]; then
    "$@"
  else
    sudo "$@"
  fi
}

launch_agent_path="$HOME/Library/LaunchAgents/$service_name.plist"
systemd_unit_path="/etc/systemd/system/$service_name.service"

macos_plist() {
  cat <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>$service_name</string>
  <key>ProgramArguments</key>
  <array>
    <string>$binary_path</string>
    <string>--port</string>
    <string>$port</string>
    <string>--log-dir</string>
    <string>$log_dir</string>
  </array>
  <key>WorkingDirectory</key>
  <string>$root_dir</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>$log_dir/service.stdout.log</string>
  <key>StandardErrorPath</key>
  <string>$log_dir/service.stderr.log</string>
</dict>
</plist>
EOF
}

linux_unit() {
  local user_name
  user_name="${ROUTER_SERVICE_USER:-$(id -un)}"
  cat <<EOF
[Unit]
Description=Router service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$user_name
WorkingDirectory=$root_dir
ExecStart=$binary_path --port $port --log-dir $log_dir
Restart=always
RestartSec=5
Environment=ROUTER_PORT=$port
Environment=ROUTER_LOG_DIR=$log_dir

[Install]
WantedBy=multi-user.target
EOF
}

macos_install() {
  require_command launchctl
  require_binary
  mkdir -p "$HOME/Library/LaunchAgents" "$log_dir"
  launchctl bootout "gui/$(id -u)/$service_name" >/dev/null 2>&1 || true
  macos_plist > "$launch_agent_path"
  launchctl bootstrap "gui/$(id -u)" "$launch_agent_path"
  launchctl enable "gui/$(id -u)/$service_name" >/dev/null 2>&1 || true
  echo "Installed and started $service_name ($launch_agent_path)."
}

macos_uninstall() {
  require_command launchctl
  launchctl bootout "gui/$(id -u)/$service_name" >/dev/null 2>&1 || true
  launchctl disable "gui/$(id -u)/$service_name" >/dev/null 2>&1 || true
  rm -f "$launch_agent_path"
  echo "Uninstalled $service_name."
}

macos_start() {
  require_command launchctl
  [[ -f "$launch_agent_path" ]] || die "Service is not installed. Run: $0 install"
  if ! launchctl print "gui/$(id -u)/$service_name" >/dev/null 2>&1; then
    launchctl bootstrap "gui/$(id -u)" "$launch_agent_path"
  fi
  launchctl kickstart -k "gui/$(id -u)/$service_name"
  echo "Started $service_name."
}

macos_stop() {
  require_command launchctl
  launchctl bootout "gui/$(id -u)/$service_name" >/dev/null 2>&1 || true
  echo "Stopped $service_name."
}

macos_restart() {
  macos_stop
  macos_start
}

macos_status() {
  require_command launchctl
  launchctl print "gui/$(id -u)/$service_name" 2>/dev/null || {
    echo "$service_name is not loaded."
    return 3
  }
}

linux_install() {
  require_command systemctl
  require_binary
  mkdir -p "$log_dir"
  if [[ "$root_dir" == *$'\n'* || "$binary_path" == *$'\n'* || "$log_dir" == *$'\n'* ]]; then
    die "Paths containing newlines are not supported in a systemd unit."
  fi
  linux_unit | run_privileged tee "$systemd_unit_path" >/dev/null
  run_privileged systemctl daemon-reload
  run_privileged systemctl enable --now "$service_name.service"
  echo "Installed and started $service_name ($systemd_unit_path)."
}

linux_uninstall() {
  require_command systemctl
  run_privileged systemctl disable --now "$service_name.service" >/dev/null 2>&1 || true
  run_privileged rm -f "$systemd_unit_path"
  run_privileged systemctl daemon-reload
  echo "Uninstalled $service_name."
}

linux_start() {
  require_command systemctl
  run_privileged systemctl start "$service_name.service"
  echo "Started $service_name."
}

linux_stop() {
  require_command systemctl
  run_privileged systemctl stop "$service_name.service"
  echo "Stopped $service_name."
}

linux_restart() {
  require_command systemctl
  run_privileged systemctl restart "$service_name.service"
  echo "Restarted $service_name."
}

linux_status() {
  require_command systemctl
  run_privileged systemctl status "$service_name.service" --no-pager
}

require_safe_service_name
case "$(uname -s)" in
  Darwin)
    case "$command_name" in
      install) macos_install ;;
      uninstall) macos_uninstall ;;
      start) macos_start ;;
      stop) macos_stop ;;
      restart) macos_restart ;;
      status) macos_status ;;
      *) die "Usage: $0 {install|uninstall|start|stop|restart|status}" ;;
    esac
    ;;
  Linux)
    case "$command_name" in
      install) linux_install ;;
      uninstall) linux_uninstall ;;
      start) linux_start ;;
      stop) linux_stop ;;
      restart) linux_restart ;;
      status) linux_status ;;
      *) die "Usage: $0 {install|uninstall|start|stop|restart|status}" ;;
    esac
    ;;
  *)
    die "Unsupported OS: $(uname -s). Use scripts/service.ps1 on Windows."
    ;;
esac
