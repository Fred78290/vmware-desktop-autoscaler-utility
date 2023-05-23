#!/bin/bash
OSDISTRO=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$([[ "$(uname -m)" =~ arm64|aarch64 ]] && echo -n arm64 || echo -n amd64)

curl -Ls https://github.com/Fred78290/vmware-desktop-autoscaler-utility/releases/download/v0.1.0/vmware-desktop-autoscaler-utility-${OSDISTRO}-${ARCH} -O /tmp/vmware-desktop-autoscaler-utility

chmod +x /tmp/vmware-desktop-autoscaler-utility

sudo mv /tmp/vmware-desktop-autoscaler-utility /usr/local/bin/vmware-desktop-autoscaler-utility

vmware-desktop-autoscaler-utility certificate generate
vmware-desktop-autoscaler-utility service install