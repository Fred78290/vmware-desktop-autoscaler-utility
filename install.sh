#!/bin/bash
OSDISTRO=$(uname -s)
ARCH=$([[ "$(uname -m)" =~ arm64|aarch64 ]] && echo -n arm64 || echo -n amd64)
