#!/bin/bash
sudo rm -rf out

VERSION=v0.1.0

make -e TAG=$VERSION all
