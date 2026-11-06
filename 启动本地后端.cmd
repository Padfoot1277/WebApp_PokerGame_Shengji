@echo off
cd /d "%~dp0"
echo Computer IPv4 addresses:
ipconfig | findstr /c:"IPv4"
echo.
echo Starting Shengji backend on port 8080. Keep this window open.
go run ./cmd/server
pause
