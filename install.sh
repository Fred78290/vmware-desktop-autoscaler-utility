#!/bin/bash
OSDISTRO=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$([[ "$(uname -m)" =~ arm64|aarch64 ]] && echo -n arm64 || echo -n amd64)
TAG_NAME=$(curl -s https://api.github.com/repos/Fred78290/vmware-desktop-autoscaler-utility/releases/latest | grep tag_name | cut -d : -f 2,3 | tr -d '\", ')

curl -Ls https://github.com/Fred78290/vmware-desktop-autoscaler-utility/releases/download/${TAG_NAME}/vmware-desktop-autoscaler-utility-${OSDISTRO}-${ARCH} -o /tmp/vmware-desktop-autoscaler-utility

chmod +x /tmp/vmware-desktop-autoscaler-utility

if [ ${OSDISTRO} == "linux" ]; then
	TARGET_DIR=/usr/local/bin
	sudo mv /tmp/vmware-desktop-autoscaler-utility ${TARGET_DIR}
else
	TARGET_DIR=${HOME}/Library/DesktopAutoscalerUtility
	mkdir -p ${TARGET_DIR}
	mv /tmp/vmware-desktop-autoscaler-utility ${TARGET_DIR}
fi

${TARGET_DIR}/vmware-desktop-autoscaler-utility certificate generate
${TARGET_DIR}/vmware-desktop-autoscaler-utility service install
