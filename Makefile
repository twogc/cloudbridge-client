# CloudBridge Client Makefile
# Multi-platform build system

.NOTPARALLEL:

VERSION := $(shell cat tag.txt | tr -d '\n\r' | xargs)
DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Build flags
VERSION_FLAGS := -X "main.Version=$(VERSION)" -X "main.BuildTime=$(DATE)"
LINK_FLAGS := -s -w

# Target OS and Architecture
TARGET_OS ?= linux
TARGET_ARCH ?= amd64
TARGET_ARM ?= 

# Binary name
BINARY_NAME := cloudbridge-client

# Build tags
GO_BUILD_TAGS := 

# CGO settings
CGO_ENABLED ?= 0

# Output directory
OUTPUT_DIR := dist

# Package settings
DEB_PACKAGE_NAME := cloudbridge-client
RPM_PACKAGE_NAME := cloudbridge-client

# Windows settings
WIX_VERSION := $(shell echo $(VERSION) | sed 's/v//')

# Build environment
export CGO_ENABLED
export GOOS=$(TARGET_OS)
export GOARCH=$(TARGET_ARCH)
export GOARM=$(TARGET_ARM)

# Default target
.PHONY: all
all: clean build

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(OUTPUT_DIR)
	rm -f $(BINARY_NAME)
	rm -f *.deb *.rpm *.msi *.dmg *.tar.gz *.zip

# Build binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) for $(TARGET_OS)/$(TARGET_ARCH)..."
	go build -tags "$(GO_BUILD_TAGS)" -ldflags "$(VERSION_FLAGS) $(LINK_FLAGS)" -o $(BINARY_NAME) ./cmd/cloudbridge-client

# Build for Linux
.PHONY: build-linux
build-linux:
	@$(MAKE) build TARGET_OS=linux TARGET_ARCH=amd64
	@$(MAKE) build TARGET_OS=linux TARGET_ARCH=arm64
	@$(MAKE) build TARGET_OS=linux TARGET_ARCH=386
	@$(MAKE) build TARGET_OS=linux TARGET_ARCH=arm TARGET_ARM=5
	@$(MAKE) build TARGET_OS=linux TARGET_ARCH=arm TARGET_ARM=7

# Build for Windows
.PHONY: build-windows
build-windows:
	@$(MAKE) build TARGET_OS=windows TARGET_ARCH=amd64
	@$(MAKE) build TARGET_OS=windows TARGET_ARCH=386
	@$(MAKE) build TARGET_OS=windows TARGET_ARCH=arm64

# Build for macOS
.PHONY: build-darwin
build-darwin:
	@$(MAKE) build TARGET_OS=darwin TARGET_ARCH=amd64
	@$(MAKE) build TARGET_OS=darwin TARGET_ARCH=arm64

# Build all platforms
.PHONY: build-all
build-all: build-linux build-windows build-darwin

# Create Linux packages
.PHONY: package-linux
package-linux:
	@echo "Creating Linux packages..."
	@mkdir -p $(OUTPUT_DIR)
	
	# Create DEB packages
	@for arch in amd64 arm64 386 arm5 arm7; do \
		$(MAKE) build TARGET_OS=linux TARGET_ARCH=$$arch; \
		$(MAKE) deb-package ARCH=$$arch; \
		mv *.deb $(OUTPUT_DIR)/; \
	done
	
	# Create RPM packages
	@for arch in amd64 arm64 386 arm5 arm7; do \
		$(MAKE) build TARGET_OS=linux TARGET_ARCH=$$arch; \
		$(MAKE) rpm-package ARCH=$$arch; \
		mv *.rpm $(OUTPUT_DIR)/; \
	done

# Create Windows packages
.PHONY: package-windows
package-windows:
	@echo "Creating Windows packages..."
	@mkdir -p $(OUTPUT_DIR)
	
	@for arch in amd64 386 arm64; do \
		$(MAKE) build TARGET_OS=windows TARGET_ARCH=$$arch; \
		$(MAKE) msi-package ARCH=$$arch; \
		mv *.msi $(OUTPUT_DIR)/; \
	done

# Create macOS packages
.PHONY: package-darwin
package-darwin:
	@echo "Creating macOS packages..."
	@mkdir -p $(OUTPUT_DIR)
	
	@for arch in amd64 arm64; do \
		$(MAKE) build TARGET_OS=darwin TARGET_ARCH=$$arch; \
		$(MAKE) dmg-package ARCH=$$arch; \
		mv *.dmg $(OUTPUT_DIR)/; \
	done

# Create all packages
.PHONY: package-all
package-all: package-linux package-windows package-darwin

# DEB package
.PHONY: deb-package
deb-package:
	@echo "Creating DEB package for $(ARCH)..."
	@mkdir -p debian/DEBIAN
	@mkdir -p debian/usr/bin
	@mkdir -p debian/usr/share/doc/cloudbridge-client
	@mkdir -p debian/etc/systemd/system
	@mkdir -p debian/opt/cloudbridge-client
	
	# Copy binary
	@cp $(BINARY_NAME) debian/usr/bin/
	@chmod +x debian/usr/bin/$(BINARY_NAME)
	
	# Copy systemd service
	@cp cloudbridge-client.service debian/etc/systemd/system/
	
	# Copy documentation
	@cp README.md debian/usr/share/doc/cloudbridge-client/
	@cp LICENSE debian/usr/share/doc/cloudbridge-client/
	@cp install.sh debian/opt/cloudbridge-client/
	@chmod +x debian/opt/cloudbridge-client/install.sh
	
	# Create control file
	@echo "Package: $(DEB_PACKAGE_NAME)" > debian/DEBIAN/control
	@echo "Version: $(VERSION)" >> debian/DEBIAN/control
	@echo "Architecture: $(ARCH)" >> debian/DEBIAN/control
	@echo "Maintainer: 2GC Dev <dev@2gc.ru>" >> debian/DEBIAN/control
	@echo "Description: CloudBridge Relay Client" >> debian/DEBIAN/control
	@echo " Cross-platform P2P mesh networking client" >> debian/DEBIAN/control
	
	# Build package
	@dpkg-deb --build debian $(BINARY_NAME)_$(VERSION)_$(ARCH).deb
	@rm -rf debian

# RPM package
.PHONY: rpm-package
rpm-package:
	@echo "Creating RPM package for $(ARCH)..."
	@mkdir -p rpmbuild/BUILD
	@mkdir -p rpmbuild/BUILDROOT
	@mkdir -p rpmbuild/RPMS
	@mkdir -p rpmbuild/SOURCES
	@mkdir -p rpmbuild/SPECS
	
	# Copy files
	@cp $(BINARY_NAME) rpmbuild/SOURCES/
	@cp cloudbridge-client.service rpmbuild/SOURCES/
	@cp README.md rpmbuild/SOURCES/
	@cp LICENSE rpmbuild/SOURCES/
	@cp install.sh rpmbuild/SOURCES/
	@chmod +x rpmbuild/SOURCES/install.sh
	
	# Create spec file
	@echo "Name: $(RPM_PACKAGE_NAME)" > rpmbuild/SPECS/cloudbridge-client.spec
	@echo "Version: $(shell echo $(VERSION) | sed 's/v//')" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "Release: 1" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "Architecture: $(ARCH)" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "Summary: CloudBridge Relay Client" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "License: Apache-2.0" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "URL: https://github.com/2gc-dev/relay-client" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "Source0: $(BINARY_NAME)" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "Source1: cloudbridge-client.service" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "Source2: README.md" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "Source3: LICENSE" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "Source4: install.sh" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "%description" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "Cross-platform P2P mesh networking client" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "%files" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "/usr/bin/$(BINARY_NAME)" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "/etc/systemd/system/cloudbridge-client.service" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "/usr/share/doc/cloudbridge-client/README.md" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "/usr/share/doc/cloudbridge-client/LICENSE" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "/opt/cloudbridge-client/install.sh" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "%pre" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "getent group cloudbridge >/dev/null || groupadd -r cloudbridge" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "getent passwd cloudbridge >/dev/null || useradd -r -g cloudbridge -d /opt/cloudbridge-client -s /bin/false cloudbridge" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "%post" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "systemctl daemon-reload" >> rpmbuild/SPECS/cloudbridge-client.spec
	@echo "systemctl enable cloudbridge-client" >> rpmbuild/SPECS/cloudbridge-client.spec
	
	# Build package
	@rpmbuild --define "_topdir $(PWD)/rpmbuild" -bb rpmbuild/SPECS/cloudbridge-client.spec
	@mv rpmbuild/RPMS/*/$(RPM_PACKAGE_NAME)-*.rpm .
	@rm -rf rpmbuild

# DEB package (Linux)
.PHONY: deb-package
deb-package:
	@echo "Creating DEB package for $(ARCH)..."
	@mkdir -p deb-build
	@cp $(BINARY_NAME) deb-build/
	@cp README.md deb-build/
	@cp LICENSE deb-build/
	@cp cloudbridge-client.service deb-build/
	@cp install.sh deb-build/
	@cp config.yaml deb-build/
	
	# Create DEB package using fpm
	@fpm -C deb-build -s dir -t deb \
		--description 'CloudBridge Relay Client for P2P mesh networking' \
		--vendor '2GC Dev' \
		--license 'MIT License' \
		--url 'https://github.com/2gc-dev/relay-client' \
		-m '2GC Dev <dev@2gc.ru>' \
		-a $(ARCH) -v $(VERSION) -n $(BINARY_NAME) \
		--after-install install.sh \
		$(BINARY_NAME)=/usr/bin/ \
		README.md=/usr/share/doc/$(BINARY_NAME)/ \
		LICENSE=/usr/share/doc/$(BINARY_NAME)/ \
		cloudbridge-client.service=/etc/systemd/system/ \
		config.yaml=/etc/$(BINARY_NAME)/ \
		install.sh=/usr/share/$(BINARY_NAME)/
	@rm -rf deb-build

# RPM package (Linux)
.PHONY: rpm-package
rpm-package:
	@echo "Creating RPM package for $(ARCH)..."
	@mkdir -p rpm-build
	@cp $(BINARY_NAME) rpm-build/
	@cp README.md rpm-build/
	@cp LICENSE rpm-build/
	@cp cloudbridge-client.service rpm-build/
	@cp install.sh rpm-build/
	@cp config.yaml rpm-build/
	
	# Create RPM package using fpm
	@fpm -C rpm-build -s dir -t rpm \
		--description 'CloudBridge Relay Client for P2P mesh networking' \
		--vendor '2GC Dev' \
		--license 'MIT License' \
		--url 'https://github.com/2gc-dev/relay-client' \
		-m '2GC Dev <dev@2gc.ru>' \
		-a $(ARCH) -v $(VERSION) -n $(BINARY_NAME) \
		--after-install install.sh \
		$(BINARY_NAME)=/usr/bin/ \
		README.md=/usr/share/doc/$(BINARY_NAME)/ \
		LICENSE=/usr/share/doc/$(BINARY_NAME)/ \
		cloudbridge-client.service=/etc/systemd/system/ \
		config.yaml=/etc/$(BINARY_NAME)/ \
		install.sh=/usr/share/$(BINARY_NAME)/
	@rm -rf rpm-build

# PKG package (macOS)
.PHONY: pkg-package
pkg-package:
	@echo "Creating PKG package for $(ARCH)..."
	@mkdir -p pkg-build
	@cp $(BINARY_NAME) pkg-build/
	@cp README.md pkg-build/
	@cp LICENSE pkg-build/
	@cp install.sh pkg-build/
	@cp config.yaml pkg-build/
	
	# Create PKG package using fpm
	@fpm -C pkg-build -s dir -t osxpkg \
		--description 'CloudBridge Relay Client for P2P mesh networking' \
		--vendor '2GC Dev' \
		--license 'MIT License' \
		--url 'https://github.com/2gc-dev/relay-client' \
		-m '2GC Dev <dev@2gc.ru>' \
		-a $(ARCH) -v $(VERSION) -n $(BINARY_NAME) \
		$(BINARY_NAME)=/usr/local/bin/ \
		README.md=/usr/local/share/doc/$(BINARY_NAME)/ \
		LICENSE=/usr/local/share/doc/$(BINARY_NAME)/ \
		config.yaml=/usr/local/etc/$(BINARY_NAME)/ \
		install.sh=/usr/local/share/$(BINARY_NAME)/
	@rm -rf pkg-build

# MSI package (Windows)
.PHONY: msi-package
msi-package:
	@echo "Creating MSI package for $(ARCH)..."
	@mkdir -p msi-build
	@cp $(BINARY_NAME).exe msi-build/
	@cp README.md msi-build/
	@cp LICENSE msi-build/
	@cp windows-service.md msi-build/
	@cp MSI_INSTALLATION.md msi-build/
	@cp install.sh msi-build/
	
	# Generate UUIDs for WiX
	@PRODUCT_ID=$$(uuidgen | tr '[:lower:]' '[:upper:]'); \
	UPGRADE_ID=$$(uuidgen | tr '[:lower:]' '[:upper:]'); \
	MAIN_GUID=$$(uuidgen | tr '[:lower:]' '[:upper:]'); \
	DOC_GUID=$$(uuidgen | tr '[:lower:]' '[:upper:]'); \
	MENU_GUID=$$(uuidgen | tr '[:lower:]' '[:upper:]'); \
	echo "Product ID: $$PRODUCT_ID"; \
	echo "Upgrade ID: $$UPGRADE_ID"; \
	echo "Main GUID: $$MAIN_GUID"; \
	echo "Doc GUID: $$DOC_GUID"; \
	echo "Menu GUID: $$MENU_GUID"
	
	# Create WiX source file with enhanced features
	@PRODUCT_ID=$$(uuidgen | tr '[:lower:]' '[:upper:]'); \
	UPGRADE_ID=$$(uuidgen | tr '[:lower:]' '[:upper:]'); \
	MAIN_GUID=$$(uuidgen | tr '[:lower:]' '[:upper:]'); \
	DOC_GUID=$$(uuidgen | tr '[:lower:]' '[:upper:]'); \
	MENU_GUID=$$(uuidgen | tr '[:lower:]' '[:upper:]'); \
	SERVICE_GUID=$$(uuidgen | tr '[:lower:]' '[:upper:]'); \
	CONFIG_GUID=$$(uuidgen | tr '[:lower:]' '[:upper:]'); \
	echo "<?xml version=\"1.0\"?>" > msi-build/cloudbridge-client.wxs; \
	echo "<Wix xmlns=\"http://schemas.microsoft.com/wix/2006/wi\">" >> msi-build/cloudbridge-client.wxs; \
	echo "  <Product Id=\"$$PRODUCT_ID\" Name=\"CloudBridge Client\" Version=\"$(WIX_VERSION)\" Manufacturer=\"2GC Dev\" Language=\"1033\" UpgradeCode=\"$$UPGRADE_ID\">" >> msi-build/cloudbridge-client.wxs; \
	echo "    <Package InstallerVersion=\"200\" Compressed=\"yes\" InstallScope=\"perMachine\" Comments=\"CloudBridge Relay Client for P2P mesh networking\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "    <Media Id=\"1\" Cabinet=\"product.cab\" EmbedCab=\"yes\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "    <MajorUpgrade DowngradeErrorMessage=\"A later version of CloudBridge Client is already installed.\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "    <Property Id=\"WIX_ACCOUNT_LOCALSYSTEM\">NT AUTHORITY\\LocalService</Property>" >> msi-build/cloudbridge-client.wxs; \
	echo "    <Directory Id=\"TARGETDIR\" Name=\"SourceDir\">" >> msi-build/cloudbridge-client.wxs; \
	echo "      <Directory Id=\"ProgramFilesFolder\">" >> msi-build/cloudbridge-client.wxs; \
	echo "        <Directory Id=\"INSTALLFOLDER\" Name=\"CloudBridge Client\">" >> msi-build/cloudbridge-client.wxs; \
	echo "          <Component Id=\"MainExecutable\" Guid=\"$$MAIN_GUID\">" >> msi-build/cloudbridge-client.wxs; \
	echo "            <File Id=\"cloudbridge-client.exe\" Source=\"cloudbridge-client.exe\" KeyPath=\"yes\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "          </Component>" >> msi-build/cloudbridge-client.wxs; \
	echo "          <Component Id=\"ConfigFile\" Guid=\"$$CONFIG_GUID\">" >> msi-build/cloudbridge-client.wxs; \
	echo "            <File Id=\"config.yaml\" Source=\"config.yaml\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "          </Component>" >> msi-build/cloudbridge-client.wxs; \
	echo "          <Component Id=\"WindowsService\" Guid=\"$$SERVICE_GUID\">" >> msi-build/cloudbridge-client.wxs; \
	echo "            <ServiceInstall Id=\"CloudBridgeService\" Type=\"ownProcess\" Name=\"CloudBridgeClient\" DisplayName=\"CloudBridge Relay Client\" Description=\"CloudBridge Relay Client for P2P mesh networking\" Start=\"auto\" Account=\"LocalSystem\" ErrorControl=\"normal\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "            <ServiceControl Id=\"CloudBridgeService\" Start=\"install\" Stop=\"both\" Remove=\"uninstall\" Name=\"CloudBridgeClient\" Wait=\"yes\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "          </Component>" >> msi-build/cloudbridge-client.wxs; \
	echo "          <Component Id=\"Documentation\" Guid=\"$$DOC_GUID\">" >> msi-build/cloudbridge-client.wxs; \
	echo "            <File Id=\"README.md\" Source=\"README.md\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "            <File Id=\"LICENSE\" Source=\"LICENSE\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "            <File Id=\"windows-service.md\" Source=\"windows-service.md\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "            <File Id=\"MSI_INSTALLATION.md\" Source=\"MSI_INSTALLATION.md\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "            <File Id=\"install.sh\" Source=\"install.sh\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "          </Component>" >> msi-build/cloudbridge-client.wxs; \
	echo "        </Directory>" >> msi-build/cloudbridge-client.wxs; \
	echo "      </Directory>" >> msi-build/cloudbridge-client.wxs; \
	echo "      <Directory Id=\"ProgramMenuFolder\">" >> msi-build/cloudbridge-client.wxs; \
	echo "        <Directory Id=\"ProgramMenuDir\" Name=\"CloudBridge Client\">" >> msi-build/cloudbridge-client.wxs; \
	echo "          <Component Id=\"ProgramMenuDir\" Guid=\"$$MENU_GUID\">" >> msi-build/cloudbridge-client.wxs; \
	echo "            <RemoveFolder Id=\"ProgramMenuDir\" On=\"uninstall\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "            <RegistryValue Root=\"HKCU\" Key=\"Software\\CloudBridge Client\" Name=\"installed\" Type=\"integer\" Value=\"1\" KeyPath=\"yes\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "          </Component>" >> msi-build/cloudbridge-client.wxs; \
	echo "        </Directory>" >> msi-build/cloudbridge-client.wxs; \
	echo "      </Directory>" >> msi-build/cloudbridge-client.wxs; \
	echo "    </Directory>" >> msi-build/cloudbridge-client.wxs; \
	echo "    <Feature Id=\"ProductFeature\" Title=\"CloudBridge Client\" Level=\"1\">" >> msi-build/cloudbridge-client.wxs; \
	echo "      <ComponentRef Id=\"MainExecutable\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "      <ComponentRef Id=\"ConfigFile\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "      <ComponentRef Id=\"WindowsService\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "      <ComponentRef Id=\"Documentation\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "      <ComponentRef Id=\"ProgramMenuDir\" />" >> msi-build/cloudbridge-client.wxs; \
	echo "    </Feature>" >> msi-build/cloudbridge-client.wxs; \
	echo "  </Product>" >> msi-build/cloudbridge-client.wxs; \
	echo "</Wix>" >> msi-build/cloudbridge-client.wxs
	
	# Build MSI using wixl (simpler than WiX Toolset)
	@wixl --define Version=$(WIX_VERSION) --define Path=$(BINARY_NAME).exe --output $(BINARY_NAME)_$(VERSION)_windows_$(ARCH).msi msi-build/cloudbridge-client.wxs
	@rm -rf msi-build

# DMG package (macOS)
.PHONY: dmg-package
dmg-package:
	@echo "Creating DMG package for $(ARCH)..."
	@mkdir -p dmg-build/CloudBridge\ Client.app/Contents/MacOS
	@mkdir -p dmg-build/CloudBridge\ Client.app/Contents/Resources
	@cp $(BINARY_NAME) dmg-build/CloudBridge\ Client.app/Contents/MacOS/
	@chmod +x dmg-build/CloudBridge\ Client.app/Contents/MacOS/$(BINARY_NAME)
	@cp README.md dmg-build/
	@cp LICENSE dmg-build/
	@cp install.sh dmg-build/
	@chmod +x dmg-build/install.sh
	
	# Create Info.plist
	@echo "<?xml version=\"1.0\" encoding=\"UTF-8\"?>" > dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	@echo "<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">" >> dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	@echo "<plist version=\"1.0\">" >> dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	@echo "<dict>" >> dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	@echo "  <key>CFBundleExecutable</key>" >> dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	@echo "  <string>$(BINARY_NAME)</string>" >> dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	@echo "  <key>CFBundleIdentifier</key>" >> dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	@echo "  <string>ru.2gc.cloudbridge-client</string>" >> dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	@echo "  <key>CFBundleName</key>" >> dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	@echo "  <string>CloudBridge Client</string>" >> dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	@echo "  <key>CFBundleVersion</key>" >> dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	@echo "  <string>$(VERSION)</string>" >> dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	@echo "</dict>" >> dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	@echo "</plist>" >> dmg-build/CloudBridge\ Client.app/Contents/Info.plist
	
	# Create DMG
	@hdiutil create -volname "CloudBridge Client" -srcfolder dmg-build -ov -format UDZO $(BINARY_NAME)_$(VERSION)_darwin_$(ARCH).dmg
	@rm -rf dmg-build

# Create archives
.PHONY: archive
archive:
	@echo "Creating archives..."
	@mkdir -p $(OUTPUT_DIR)
	
	@for os in linux windows darwin; do \
		for arch in amd64 arm64 386; do \
			if [ "$$os" = "darwin" ] && [ "$$arch" = "386" ]; then continue; fi; \
			if [ "$$os" = "windows" ] && [ "$$arch" = "arm64" ]; then continue; fi; \
			$(MAKE) build TARGET_OS=$$os TARGET_ARCH=$$arch; \
			if [ "$$os" = "windows" ]; then \
				mv $(BINARY_NAME) $(BINARY_NAME).exe; \
				zip $(BINARY_NAME)_$(VERSION)_$$os_$$arch.zip $(BINARY_NAME).exe README.md LICENSE cloudbridge-client.service windows-service.md install.sh Architecture.md; \
				rm $(BINARY_NAME).exe; \
			else \
				tar -czf $(BINARY_NAME)_$(VERSION)_$$os_$$arch.tar.gz $(BINARY_NAME) README.md LICENSE cloudbridge-client.service windows-service.md install.sh Architecture.md; \
			fi; \
			mv $(BINARY_NAME)_$(VERSION)_$$os_$$arch.* $(OUTPUT_DIR)/; \
		done; \
	done

# Create checksums
.PHONY: checksums
checksums:
	@echo "Creating checksums..."
	@cd $(OUTPUT_DIR) && sha256sum * > checksums.txt

# Full release build
.PHONY: release
release: clean build-all package-all archive checksums
	@echo "Release build complete!"
	@echo "Artifacts in $(OUTPUT_DIR)/"
	@ls -la $(OUTPUT_DIR)/

# Help
.PHONY: help
help:
	@echo "CloudBridge Client Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build          - Build binary for current platform"
	@echo "  build-linux    - Build for all Linux architectures"
	@echo "  build-windows  - Build for all Windows architectures"
	@echo "  build-darwin   - Build for all macOS architectures"
	@echo "  build-all      - Build for all platforms"
	@echo "  package-linux  - Create Linux packages (DEB/RPM)"
	@echo "  package-windows - Create Windows packages (MSI)"
	@echo "  package-darwin - Create macOS packages (DMG)"
	@echo "  package-all    - Create all packages"
	@echo "  archive        - Create distribution archives"
	@echo "  checksums      - Create checksums file"
	@echo "  release        - Full release build"
	@echo "  clean          - Clean build artifacts"
	@echo "  help           - Show this help"
	@echo ""
	@echo "Variables:"
	@echo "  TARGET_OS      - Target operating system (linux/windows/darwin)"
	@echo "  TARGET_ARCH    - Target architecture (amd64/arm64/386/arm)"
	@echo "  TARGET_ARM     - ARM version (5/6/7) for ARM builds"
	@echo "  CGO_ENABLED    - Enable CGO (0/1)"
	@echo "  OUTPUT_DIR     - Output directory (default: dist)"