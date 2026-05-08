@echo off
setlocal
cd /d "%~dp0"

echo ===================================================
echo   DNSE MT5 Connector - One Click Local Build
echo ===================================================
echo.
echo This build does not commit, push, create GitHub Release,
echo or deploy files into MT5.
echo.

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0release_one_click.ps1" -SkipGit -SkipRelease -SkipDeployMt5

if errorlevel 1 (
  echo.
  echo Build local failed. Please review the messages above.
  pause
  exit /b 1
)

echo.
echo Local build completed.
echo Output folder: %~dp0dist
pause
