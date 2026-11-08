@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\deploy-reset.ps1"
if errorlevel 1 echo Deployment failed. See details above.
pause
