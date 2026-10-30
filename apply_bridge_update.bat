@echo off
setlocal
cd /d "%~dp0"

echo Dang tat bridge cu...
taskkill /IM bridge.exe /F >nul 2>&1
timeout /t 2 /nobreak >nul

echo Dang cap nhat bridge.exe...
copy /Y bridge_new.exe bridge.exe >nul
if errorlevel 1 (
  echo Cap nhat that bai. Hay dong bridge.exe roi chay lai file nay.
  pause
  exit /b 1
)

echo Dang mo lai bridge...
start "" /min "%~dp0bridge.exe"

echo Da cap nhat xong. Co the quay lai MT5 va doi EA ket noi lai.
pause
