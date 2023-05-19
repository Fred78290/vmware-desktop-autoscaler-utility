#!/bin/bash
sudo rm -rf out

VERSION=v0.1.0
REGISTRY=devregistry.aldunelabs.com

make -e REGISTRY=$REGISTRY -e TAG=$VERSION install
