#!/bin/bash

set -e

echo "[+] Building wireguird binary..."
GOOS=linux GOARCH=amd64 go build -o wireguird

echo "[+] Copying binary to package structure..."
cp wireguird wireguird_1.0.0/usr/local/bin/

echo "[+] Building .deb package..."
dpkg-deb --build wireguird_1.0.0

echo "[+] Done. Output: wireguird_1.0.0.deb"
