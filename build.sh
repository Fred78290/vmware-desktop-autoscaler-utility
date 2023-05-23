#!/bin/bash
sudo rm -rf out

VERSION=v0.1.0

make -e TAG=$VERSION all

mkdir -p release

cp out/linux/amd64/vmware-desktop-autoscaler-utility ./release/vmware-desktop-autoscaler-utility-linux-amd64
cp out/linux/arm64/vmware-desktop-autoscaler-utility ./release/vmware-desktop-autoscaler-utility-linux-arm64
cp out/darwin/amd64/vmware-desktop-autoscaler-utility ./release/vmware-desktop-autoscaler-utility-darwin-amd64
cp out/darwin/arm64/vmware-desktop-autoscaler-utility ./release/vmware-desktop-autoscaler-utility-darwin-arm64
