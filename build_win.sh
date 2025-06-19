#!/bin/bash
set -e

APP_NAME="wireguird"
ARCH="amd64"
FYNE_OUTPUT="fyne-cross/dist/windows-${ARCH}"
BUILD_DIR="win-dist"
DLL_PATH="./ddl/mesa3d-25.1.3-debug-mingw/x64"
WG_EXE="./wg.exe"  # ← путь к wg.exe рядом с этим скриптом
FINAL_ZIP="${APP_NAME}_windows.zip"

echo "[+] Building ${APP_NAME}.exe for Windows..."
fyne-cross windows -arch=${ARCH}

echo "[+] Cleaning previous output..."
rm -rf "${BUILD_DIR}" "$FINAL_ZIP"
mkdir -p "${BUILD_DIR}"

echo "[+] Extracting built executable..."
unzip -o "${FYNE_OUTPUT}/WireGuird VPN.exe.zip" -d "${BUILD_DIR}"

EXE_FOUND=$(find "${BUILD_DIR}" -iname '*.exe' | head -n1)
mv "$EXE_FOUND" "${BUILD_DIR}/${APP_NAME}.exe"

echo "[+] Copying MESA DLLs..."
cp "${DLL_PATH}"/*.dll "${BUILD_DIR}/"

echo "[+] Copying wg.exe..."
cp "${WG_EXE}" "${BUILD_DIR}/"

echo "[+] Creating archive ${FINAL_ZIP}..."
zip -j "$FINAL_ZIP" "${BUILD_DIR}/"*

echo "[✓] Done. Archive created: ${FINAL_ZIP}"
