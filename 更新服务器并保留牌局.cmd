@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\deploy-preserve.ps1"
if errorlevel 1 echo Deployment failed. See details above.
pause
