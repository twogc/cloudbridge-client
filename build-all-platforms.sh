#!/bin/bash

# CloudBridge Client - Multi-platform Build Script
# Based on cloudflared build system

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# Get version from tag.txt
VERSION=$(cat tag.txt | tr -d '\n\r' | xargs)
log_info "Building CloudBridge Client version: $VERSION"

# Create artifacts directory
ARTIFACT_DIR="artifacts"
mkdir -p $ARTIFACT_DIR

# Build settings
export CGO_ENABLED=0
export GOEXPERIMENT=noboringcrypto

# Linux architectures
linuxArchs=("amd64" "arm64" "386" "arm5" "arm7")

log_info "Building for Linux..."
for arch in "${linuxArchs[@]}"; do
    log_info "Building Linux $arch..."
    
    # Set ARM version
    if [[ "$arch" == "arm5" ]]; then
        export GOOS=linux
        export GOARCH=arm
        export GOARM=5
    elif [[ "$arch" == "arm7" ]]; then
        export GOOS=linux
        export GOARCH=arm
        export GOARM=7
    else
        export GOOS=linux
        export GOARCH=$arch
        unset GOARM
    fi
    
    # Build binary
    go build -ldflags="-s -w -X main.Version=$VERSION -X main.BuildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')" -o cloudbridge-client ./cmd/cloudbridge-client
    
    # Create archive
    tar -czf $ARTIFACT_DIR/cloudbridge-client_${VERSION}_linux_${arch}.tar.gz \
        cloudbridge-client \
        README.md \
        LICENSE \
        cloudbridge-client.service \
        windows-service.md \
        install.sh \
        Architecture.md
    
    # Create DEB package
    log_info "Creating DEB package for $arch..."
    make deb-package ARCH=$arch
    mv cloudbridge-client_${VERSION}_${arch}.deb $ARTIFACT_DIR/cloudbridge-client_${VERSION}_linux_${arch}.deb
    
    # Create RPM package
    log_info "Creating RPM package for $arch..."
    make rpm-package ARCH=$arch
    mv cloudbridge-client-*.rpm $ARTIFACT_DIR/cloudbridge-client_${VERSION}_linux_${arch}.rpm
    
    # Clean up
    rm -f cloudbridge-client
    rm -rf debian rpmbuild
done

# Windows architectures
windowsArchs=("amd64" "386" "arm64")

log_info "Building for Windows..."
for arch in "${windowsArchs[@]}"; do
    log_info "Building Windows $arch..."
    
    export GOOS=windows
    export GOARCH=$arch
    unset GOARM
    
    # Build binary
    go build -ldflags="-s -w -X main.Version=$VERSION -X main.BuildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')" -o cloudbridge-client.exe ./cmd/cloudbridge-client
    
    # Create archive
    zip $ARTIFACT_DIR/cloudbridge-client_${VERSION}_windows_${arch}.zip \
        cloudbridge-client.exe \
        README.md \
        LICENSE \
        cloudbridge-client.service \
        windows-service.md \
        install.sh \
        Architecture.md
    
    # Create MSI package
    log_info "Creating MSI package for $arch..."
    make msi-package ARCH=$arch
    mv cloudbridge-client_${VERSION}_windows_${arch}.msi $ARTIFACT_DIR/
    
    # Clean up
    rm -f cloudbridge-client.exe
    rm -rf msi-build
done

# macOS architectures
darwinArchs=("amd64" "arm64")

log_info "Building for macOS..."
for arch in "${darwinArchs[@]}"; do
    log_info "Building macOS $arch..."
    
    export GOOS=darwin
    export GOARCH=$arch
    unset GOARM
    
    # Build binary
    go build -ldflags="-s -w -X main.Version=$VERSION -X main.BuildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')" -o cloudbridge-client ./cmd/cloudbridge-client
    
    # Create archive
    tar -czf $ARTIFACT_DIR/cloudbridge-client_${VERSION}_darwin_${arch}.tar.gz \
        cloudbridge-client \
        README.md \
        LICENSE \
        cloudbridge-client.service \
        windows-service.md \
        install.sh \
        Architecture.md
    
    # Create DMG package
    log_info "Creating DMG package for $arch..."
    make dmg-package ARCH=$arch
    mv cloudbridge-client_${VERSION}_darwin_${arch}.dmg $ARTIFACT_DIR/
    
    # Clean up
    rm -f cloudbridge-client
    rm -rf dmg-build
done

# Create checksums
log_info "Creating checksums..."
cd $ARTIFACT_DIR
sha256sum * > checksums.txt
cd ..

# Create release notes
log_info "Creating release notes..."
cat > $ARTIFACT_DIR/RELEASE_NOTES.md << EOF
# CloudBridge Client $VERSION

## Downloads

### Linux
- **DEB packages**: \`cloudbridge-client_${VERSION}_linux_*.deb\`
- **RPM packages**: \`cloudbridge-client_${VERSION}_linux_*.rpm\`
- **Archives**: \`cloudbridge-client_${VERSION}_linux_*.tar.gz\`

### Windows
- **MSI packages**: \`cloudbridge-client_${VERSION}_windows_*.msi\`
- **Archives**: \`cloudbridge-client_${VERSION}_windows_*.zip\`

### macOS
- **DMG packages**: \`cloudbridge-client_${VERSION}_darwin_*.dmg\`
- **Archives**: \`cloudbridge-client_${VERSION}_darwin_*.tar.gz\`

## Installation

### Linux
\`\`\`bash
# DEB (Ubuntu/Debian)
sudo dpkg -i cloudbridge-client_${VERSION}_linux_amd64.deb

# RPM (CentOS/RHEL/Fedora)
sudo rpm -i cloudbridge-client_${VERSION}_linux_x86_64.rpm

# Manual installation
tar -xzf cloudbridge-client_${VERSION}_linux_amd64.tar.gz
sudo ./install.sh
\`\`\`

### Windows
\`\`\`cmd
# MSI installation
msiexec /i cloudbridge-client_${VERSION}_windows_amd64.msi

# Manual installation
# Extract cloudbridge-client_${VERSION}_windows_amd64.zip
# Follow windows-service.md for service setup
\`\`\`

### macOS
\`\`\`bash
# DMG installation
open cloudbridge-client_${VERSION}_darwin_arm64.dmg

# Manual installation
tar -xzf cloudbridge-client_${VERSION}_darwin_arm64.tar.gz
sudo ./install.sh
\`\`\`

## Verification

Verify the integrity of downloaded files:

\`\`\`bash
sha256sum -c checksums.txt
\`\`\`

## Features

- Cross-platform P2P mesh networking
- QUIC transport protocol
- WebSocket fallback
- WireGuard L3-overlay support
- Enterprise-grade security
- System service integration
- Comprehensive documentation

## Documentation

- **README.md**: Quick start guide
- **Architecture.md**: System architecture
- **windows-service.md**: Windows service setup
- **install.sh**: Automated installation script

## Support

- GitHub Issues: https://github.com/2gc-dev/relay-client/issues
- Documentation: https://github.com/2gc-dev/relay-client
EOF

log_success "Build complete!"
log_info "Artifacts created in $ARTIFACT_DIR/"
log_info "Total files: $(ls -1 $ARTIFACT_DIR | wc -l)"
log_info "Total size: $(du -sh $ARTIFACT_DIR | cut -f1)"

echo ""
echo "=== Build Summary ==="
echo "Version: $VERSION"
echo "Artifacts: $ARTIFACT_DIR/"
echo "Checksums: $ARTIFACT_DIR/checksums.txt"
echo "Release Notes: $ARTIFACT_DIR/RELEASE_NOTES.md"
echo ""
echo "Files created:"
ls -la $ARTIFACT_DIR/
