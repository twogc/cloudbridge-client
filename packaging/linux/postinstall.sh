#!/usr/bin/env bash
set -e

# CloudBridge Client post-install script for Linux packages

echo "Configuring CloudBridge Client service..."

# Reload systemd daemon
systemctl daemon-reload || true

# Enable the service
systemctl enable cloudbridge-client || true

# Create log directory
mkdir -p /var/log/cloudbridge-client
chown cloudbridge-client:cloudbridge-client /var/log/cloudbridge-client 2>/dev/null || true

# Set proper permissions on config file
chmod 644 /etc/cloudbridge-client/config.yaml 2>/dev/null || true

echo "CloudBridge Client service configured successfully!"
echo "To start the service: sudo systemctl start cloudbridge-client"
echo "To check status: sudo systemctl status cloudbridge-client"
