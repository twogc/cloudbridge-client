# MSI Installation Guide

## Automatic Windows Service Installation

The CloudBridge Client MSI package automatically installs and configures a Windows service during installation.

### What the MSI Package Does

1. **Installs Files**:
   - `cloudbridge-client.exe` - Main executable
   - `config.yaml` - Default configuration template
   - Documentation files (README.md, LICENSE, etc.)

2. **Creates Windows Service**:
   - **Service Name**: `CloudBridgeClient`
   - **Display Name**: `CloudBridge Relay Client`
   - **Description**: `CloudBridge Relay Client for P2P mesh networking`
   - **Start Type**: Automatic (starts with Windows)
   - **Account**: Local System
   - **Executable**: `C:\Program Files\CloudBridge Client\cloudbridge-client.exe`
   - **Arguments**: `p2p --config "C:\Program Files\CloudBridge Client\config.yaml"`

3. **Service Management**:
   - Service is automatically started after installation
   - Service is automatically stopped and removed during uninstallation
   - Service runs in the background as a Windows service

### Installation Process

1. **Download** the MSI package from GitHub Releases
2. **Run** the MSI installer as Administrator
3. **Follow** the installation wizard
4. **Service** is automatically created and started

### Post-Installation Configuration

After installation, you need to configure the service:

1. **Edit Configuration**:
   ```
   C:\Program Files\CloudBridge Client\config.yaml
   ```

2. **Update Authentication**:
   - Set your JWT token
   - Configure server endpoints
   - Adjust other settings as needed

3. **Restart Service**:
   ```cmd
   # Using Services.msc
   # Or using command line:
   net stop CloudBridgeClient
   net start CloudBridgeClient
   ```

### Service Management Commands

```cmd
# Start service
net start CloudBridgeClient

# Stop service
net stop CloudBridgeClient

# Check service status
sc query CloudBridgeClient

# Restart service
net stop CloudBridgeClient && net start CloudBridgeClient
```

### Service Properties

- **Service Name**: `CloudBridgeClient`
- **Display Name**: `CloudBridge Relay Client`
- **Description**: `CloudBridge Relay Client for P2P mesh networking`
- **Start Type**: Automatic
- **Account**: Local System
- **Executable Path**: `C:\Program Files\CloudBridge Client\cloudbridge-client.exe`
- **Command Line**: `p2p --config "C:\Program Files\CloudBridge Client\config.yaml"`

### Logs and Monitoring

The service runs in the background and logs to:
- **Windows Event Log**: Look for "CloudBridgeClient" in Application log
- **Console Output**: Captured by Windows service manager

### Uninstallation

When you uninstall the MSI package:
1. Service is automatically stopped
2. Service is removed from Windows
3. Files are removed from the system
4. Registry entries are cleaned up

### Troubleshooting

#### Service Won't Start
1. Check the configuration file for syntax errors
2. Verify network connectivity
3. Check Windows Event Log for errors
4. Ensure the executable has proper permissions

#### Configuration Issues
1. Edit `config.yaml` in the installation directory
2. Restart the service after making changes
3. Check the service logs for specific error messages

#### Network Issues
1. Verify firewall settings
2. Check if required ports are available
3. Ensure TLS certificates are valid
4. Test connectivity to the relay server

### Security Considerations

- The service runs as Local System (highest privileges)
- Configuration files are readable by administrators
- Network traffic is encrypted (TLS)
- JWT tokens should be kept secure
- Consider using a dedicated service account for production

### Advanced Configuration

For advanced users, you can:
1. **Change Service Account**: Use a dedicated service account instead of Local System
2. **Custom Logging**: Configure custom log destinations
3. **Performance Tuning**: Adjust connection pool sizes and timeouts
4. **Security Hardening**: Implement additional security measures

### Support

For issues with the MSI installation or service configuration:
1. Check the Windows Event Log
2. Review the configuration file
3. Test network connectivity
4. Contact support with detailed logs
