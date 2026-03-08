package provisioning

// GenerateSystemdUnit generates a systemd service unit file for the edgefabric agent.
func GenerateSystemdUnit() string {
	return `[Unit]
Description=EdgeFabric Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/edgefabric --config /etc/edgefabric/config.yaml
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target`
}
