#!/usr/bin/env bash
set -e

# CloudBridge Client pre-remove script for Linux packages

echo "Stopping CloudBridge Client service..."

# Stop the service
systemctl stop cloudbridge-client || true

# Disable the service
systemctl disable cloudbridge-client || true

echo "CloudBridge Client service stopped and disabled."
