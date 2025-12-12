#!/usr/bin/env bash
set -e

# CloudBridge Client post-install script for macOS packages

echo "Configuring CloudBridge Client for macOS..."

# Create launchd plist for service
cat > /Library/LaunchDaemons/ru.2gc.cloudbridge.client.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>ru.2gc.cloudbridge.client</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/cloudbridge-client</string>
        <string>p2p</string>
        <string>--config</string>
        <string>/usr/local/etc/cloudbridge-client/config.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/cloudbridge-client.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/cloudbridge-client.log</string>
    <key>UserName</key>
    <string>root</string>
</dict>
</plist>
EOF

# Set proper permissions
chmod 644 /Library/LaunchDaemons/ru.2gc.cloudbridge.client.plist

# Create log directory
mkdir -p /var/log
touch /var/log/cloudbridge-client.log
chmod 644 /var/log/cloudbridge-client.log

# Set proper permissions on config file
chmod 644 /usr/local/etc/cloudbridge-client/config.yaml 2>/dev/null || true

echo "CloudBridge Client configured successfully!"
echo "To start the service: sudo launchctl load /Library/LaunchDaemons/ru.2gc.cloudbridge.client.plist"
echo "To check status: sudo launchctl list | grep cloudbridge"
