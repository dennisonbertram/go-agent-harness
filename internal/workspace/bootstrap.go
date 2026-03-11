package workspace

// harnessBootstrapScript returns a cloud-init user_data bash script that
// installs dependencies and starts harnessd on port 8080.
//
// The harnessd binary must be accessible on the VM. If the HARNESS_DOWNLOAD_URL
// environment variable is set at boot time, the script downloads the binary from
// that URL. Otherwise, it expects harnessd to already be present in the VM image
// at /usr/local/bin/harnessd.
func harnessBootstrapScript() string {
	return `#!/bin/bash
set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl git

# Download harnessd if HARNESS_DOWNLOAD_URL is set, otherwise use pre-installed binary
if [ -n "${HARNESS_DOWNLOAD_URL}" ]; then
    curl -fsSL "${HARNESS_DOWNLOAD_URL}" -o /usr/local/bin/harnessd
    chmod +x /usr/local/bin/harnessd
fi

# Create workspace directory
mkdir -p /workspace

# Write systemd service
cat > /etc/systemd/system/harnessd.service << 'SERVICE'
[Unit]
Description=Agent Harness Daemon
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/harnessd
Environment=HARNESS_WORKSPACE=/workspace
Environment=HARNESS_ADDR=:8080
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
SERVICE

systemctl daemon-reload
systemctl enable harnessd
systemctl start harnessd || true
`
}
