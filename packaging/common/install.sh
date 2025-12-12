#!/bin/bash

# CloudBridge Client Installation Script
# Supports Linux, macOS, and Windows (via WSL)

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
INSTALL_DIR="/opt/cloudbridge-client"
CONFIG_DIR="/etc/cloudbridge-client"
SERVICE_USER="cloudbridge"
SERVICE_GROUP="cloudbridge"
VERSION="latest"
PLATFORM=""
ARCH=""
DOWNLOAD_URL=""

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case $arch in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        i386|i686)
            ARCH="386"
            ;;
        *)
            log_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
    
    case $os in
        linux)
            PLATFORM="linux"
            ;;
        darwin)
            PLATFORM="darwin"
            ;;
        *)
            log_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
    
    log_info "Detected platform: $PLATFORM-$ARCH"
}

download_binary() {
    local version=$1
    local platform=$2
    local arch=$3
    
    if [[ "$version" == "latest" ]]; then
        DOWNLOAD_URL="https://github.com/2gc-dev/relay-client/releases/latest/download/cloudbridge-client_${platform}_${arch}.tar.gz"
    else
        DOWNLOAD_URL="https://github.com/2gc-dev/relay-client/releases/download/${version}/cloudbridge-client_${version}_${platform}_${arch}.tar.gz"
    fi
    
    log_info "Downloading from: $DOWNLOAD_URL"
    
    # Create temporary directory
    local temp_dir=$(mktemp -d)
    cd "$temp_dir"
    
    # Download and extract
    if command -v curl >/dev/null 2>&1; then
        curl -L -o "cloudbridge-client.tar.gz" "$DOWNLOAD_URL"
    elif command -v wget >/dev/null 2>&1; then
        wget -O "cloudbridge-client.tar.gz" "$DOWNLOAD_URL"
    else
        log_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
    
    tar -xzf "cloudbridge-client.tar.gz"
    
    # Verify binary
    if [[ ! -f "cloudbridge-client" ]]; then
        log_error "Binary not found in archive"
        exit 1
    fi
    
    # Make executable
    chmod +x "cloudbridge-client"
    
    # Test binary
    if ! ./cloudbridge-client version >/dev/null 2>&1; then
        log_error "Downloaded binary is not working"
        exit 1
    fi
    
    log_success "Binary downloaded and verified successfully"
    
    # Return path to extracted files
    echo "$temp_dir"
}

install_binary() {
    local temp_dir=$1
    
    log_info "Installing binary to $INSTALL_DIR"
    
    # Create directories
    sudo mkdir -p "$INSTALL_DIR"
    sudo mkdir -p "$CONFIG_DIR"
    
    # Copy binary
    sudo cp "$temp_dir/cloudbridge-client" "$INSTALL_DIR/"
    sudo chmod +x "$INSTALL_DIR/cloudbridge-client"
    
    # Copy config files if they exist
    if [[ -f "$temp_dir/config.yaml" ]]; then
        sudo cp "$temp_dir/config.yaml" "$CONFIG_DIR/config.yaml.example"
        log_info "Example config copied to $CONFIG_DIR/config.yaml.example"
    fi
    
    if [[ -f "$temp_dir/README.md" ]]; then
        sudo cp "$temp_dir/README.md" "$INSTALL_DIR/"
    fi
    
    # Create symlink for easy access
    sudo ln -sf "$INSTALL_DIR/cloudbridge-client" "/usr/local/bin/cloudbridge-client"
    
    log_success "Binary installed successfully"
}

create_user() {
    log_info "Creating service user: $SERVICE_USER"
    
    # Check if user already exists
    if id "$SERVICE_USER" >/dev/null 2>&1; then
        log_info "User $SERVICE_USER already exists"
    else
        sudo useradd --system --no-create-home --shell /bin/false "$SERVICE_USER"
        log_success "User $SERVICE_USER created"
    fi
    
    # Set ownership
    sudo chown -R "$SERVICE_USER:$SERVICE_GROUP" "$INSTALL_DIR"
    sudo chown -R "$SERVICE_USER:$SERVICE_GROUP" "$CONFIG_DIR"
}

install_systemd_service() {
    log_info "Installing systemd service"
    
    # Copy service file
    sudo cp "cloudbridge-client.service" "/etc/systemd/system/"
    
    # Reload systemd
    sudo systemctl daemon-reload
    
    # Enable service (but don't start yet)
    sudo systemctl enable cloudbridge-client
    
    log_success "Systemd service installed and enabled"
}

setup_config() {
    log_info "Setting up configuration"
    
    # Create config directory structure
    sudo mkdir -p "$CONFIG_DIR/config.d"
    
    # Set permissions
    sudo chmod 755 "$CONFIG_DIR"
    sudo chmod 700 "$CONFIG_DIR/config.d"
    
    # Create example config if it doesn't exist
    if [[ ! -f "$CONFIG_DIR/config.yaml" ]]; then
        if [[ -f "$CONFIG_DIR/config.yaml.example" ]]; then
            sudo cp "$CONFIG_DIR/config.yaml.example" "$CONFIG_DIR/config.yaml"
            log_info "Created config.yaml from example"
        else
            log_warning "No example config found. You'll need to create config.yaml manually"
        fi
    fi
    
    log_info "Configuration setup complete"
    log_info "Edit $CONFIG_DIR/config.yaml to configure your client"
}

show_next_steps() {
    log_success "Installation completed successfully!"
    echo
    echo "Next steps:"
    echo "1. Edit configuration: sudo nano $CONFIG_DIR/config.yaml"
    echo "2. Start the service: sudo systemctl start cloudbridge-client"
    echo "3. Check status: sudo systemctl status cloudbridge-client"
    echo "4. View logs: sudo journalctl -u cloudbridge-client -f"
    echo
    echo "Service management:"
    echo "  Start:   sudo systemctl start cloudbridge-client"
    echo "  Stop:    sudo systemctl stop cloudbridge-client"
    echo "  Restart: sudo systemctl restart cloudbridge-client"
    echo "  Status:  sudo systemctl status cloudbridge-client"
    echo
    echo "Configuration files:"
    echo "  Main config: $CONFIG_DIR/config.yaml"
    echo "  Service:     /etc/systemd/system/cloudbridge-client.service"
    echo "  Binary:      $INSTALL_DIR/cloudbridge-client"
    echo "  Symlink:     /usr/local/bin/cloudbridge-client"
}

cleanup() {
    if [[ -n "${temp_dir:-}" && -d "$temp_dir" ]]; then
        rm -rf "$temp_dir"
    fi
}

# Main installation function
main() {
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --version)
                VERSION="$2"
                shift 2
                ;;
            --install-dir)
                INSTALL_DIR="$2"
                shift 2
                ;;
            --config-dir)
                CONFIG_DIR="$2"
                shift 2
                ;;
            --user)
                SERVICE_USER="$2"
                shift 2
                ;;
            --group)
                SERVICE_GROUP="$2"
                shift 2
                ;;
            --help)
                echo "CloudBridge Client Installation Script"
                echo
                echo "Usage: $0 [OPTIONS]"
                echo
                echo "Options:"
                echo "  --version VERSION     Version to install (default: latest)"
                echo "  --install-dir DIR     Installation directory (default: /opt/cloudbridge-client)"
                echo "  --config-dir DIR      Configuration directory (default: /etc/cloudbridge-client)"
                echo "  --user USER           Service user (default: cloudbridge)"
                echo "  --group GROUP         Service group (default: cloudbridge)"
                echo "  --help                Show this help"
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done
    
    # Set up cleanup trap
    trap cleanup EXIT
    
    log_info "Starting CloudBridge Client installation"
    log_info "Version: $VERSION"
    log_info "Install directory: $INSTALL_DIR"
    log_info "Config directory: $CONFIG_DIR"
    
    # Check if running as root
    if [[ $EUID -eq 0 ]]; then
        log_warning "Running as root. This is not recommended for security reasons."
    fi
    
    # Detect platform
    detect_platform
    
    # Download binary
    temp_dir=$(download_binary "$VERSION" "$PLATFORM" "$ARCH")
    
    # Install binary
    install_binary "$temp_dir"
    
    # Create user
    create_user
    
    # Install systemd service
    install_systemd_service
    
    # Setup configuration
    setup_config
    
    # Show next steps
    show_next_steps
}

# Run main function
main "$@"
