@echo off
setlocal
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0tools\BUILD_VIRTUAL_SURROUND_CABLES.ps1"
if errorlevel 1 pause & exit /b 1
echo.
echo Build completed: Release\Virtual Surround Cables.exe
pause
