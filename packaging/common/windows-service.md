# Windows Service Installation

## Using NSSM (Non-Sucking Service Manager)

### 1. Download and Install NSSM
```powershell
# Download NSSM from https://nssm.cc/download
# Extract to C:\nssm\
```

### 2. Install CloudBridge Client as Service
```cmd
# Open Command Prompt as Administrator
cd C:\nssm\win64
nssm install CloudBridgeClient

# Configure the service:
# - Path: C:\cloudbridge-client\cloudbridge-client.exe
# - Arguments: p2p --config C:\cloudbridge-client\config.yaml
# - Startup directory: C:\cloudbridge-client
# - Log on: Local System account
```

### 3. Configure Service
```cmd
# Set service to auto-start
nssm set CloudBridgeClient Start SERVICE_AUTO_START

# Set service description
nssm set CloudBridgeClient Description "CloudBridge Relay Client for P2P mesh networking"

# Configure logging
nssm set CloudBridgeClient AppStdout C:\cloudbridge-client\logs\stdout.log
nssm set CloudBridgeClient AppStderr C:\cloudbridge-client\logs\stderr.log
nssm set CloudBridgeClient AppRotateFiles 1
nssm set CloudBridgeClient AppRotateOnline 1
nssm set CloudBridgeClient AppRotateBytes 1048576
```

### 4. Start Service
```cmd
# Start the service
nssm start CloudBridgeClient

# Check service status
nssm status CloudBridgeClient
```

## Using Windows Service Wrapper (Alternative)

### 1. Download winsw
```powershell
# Download winsw from https://github.com/winsw/winsw/releases
# Rename to cloudbridge-client.exe and place in service directory
```

### 2. Create service configuration
```xml
<!-- cloudbridge-client.xml -->
<service>
  <id>CloudBridgeClient</id>
  <name>CloudBridge Relay Client</name>
  <description>CloudBridge Relay Client for P2P mesh networking</description>
  <executable>C:\cloudbridge-client\cloudbridge-client.exe</executable>
  <arguments>p2p --config C:\cloudbridge-client\config.yaml</arguments>
  <workingdirectory>C:\cloudbridge-client</workingdirectory>
  <log mode="roll-by-time">
    <pattern>yyyy-MM-dd</pattern>
    <autoRollAtTime>00:00:00</autoRollAtTime>
    <zipOlderThanNumDays>5</zipOlderThanNumDays>
    <zipDateFormat>yyyy-MM-dd</zipDateFormat>
  </log>
  <onfailure action="restart" delay="10 sec"/>
  <resetfailure>1 hour</resetfailure>
</service>
```

### 3. Install and Start
```cmd
# Install service
cloudbridge-client.exe install

# Start service
cloudbridge-client.exe start

# Check status
cloudbridge-client.exe status
```

## Service Management

### Start/Stop/Restart
```cmd
# Using NSSM
nssm start CloudBridgeClient
nssm stop CloudBridgeClient
nssm restart CloudBridgeClient

# Using winsw
cloudbridge-client.exe start
cloudbridge-client.exe stop
cloudbridge-client.exe restart
```

### View Logs
```cmd
# NSSM logs
type C:\cloudbridge-client\logs\stdout.log
type C:\cloudbridge-client\logs\stderr.log

# Windows Event Log
eventvwr.msc
# Look for "CloudBridgeClient" in Application log
```

### Uninstall Service
```cmd
# NSSM
nssm remove CloudBridgeClient

# winsw
cloudbridge-client.exe uninstall
```

## Security Considerations

1. **Service Account**: Use a dedicated service account with minimal privileges
2. **File Permissions**: Restrict access to config files
3. **Network Access**: Configure Windows Firewall rules
4. **Log Rotation**: Implement log rotation to prevent disk space issues
5. **Auto-restart**: Configure automatic restart on failure

## Troubleshooting

### Common Issues
- **Permission Denied**: Run as Administrator
- **Port Conflicts**: Check if ports are already in use
- **Config Errors**: Validate config.yaml syntax
- **Network Issues**: Check firewall and network connectivity

### Debug Mode
```cmd
# Run in debug mode to see detailed logs
cloudbridge-client.exe p2p --config config.yaml --verbose --debug
```
