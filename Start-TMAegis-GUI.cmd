@echo off
setlocal
cd /d "%~dp0"
title TMAegis Desktop GUI

if not exist "%~dp0tmaegis.exe" (
    echo [!] Cannot find tmaegis.exe in "%~dp0".
    echo     Please extract the full ZIP archive before running this launcher.
    pause
    exit /b 1
)

echo [*] Starting TMAegis Desktop GUI (127.0.0.1)...
echo [*] Tip: To apply system remediation/hardening changes, right-click tmaegis.exe and select "Run as administrator".
"%~dp0tmaegis.exe" gui %*
