@echo off
setlocal
cd /d "%~dp0"

if not exist bridge.pid (
    echo Khong tim thay bridge.pid. Co the bridge da dung.
    exit /b 0
)

for /f "usebackq delims=" %%i in ("bridge.pid") do set PID=%%i
if "%PID%"=="" (
    echo bridge.pid khong hop le.
    del /q bridge.pid >nul 2>&1
    exit /b 1
)

echo Dang dung bridge PID %PID% ...
taskkill /PID %PID% /T /F
del /q bridge.pid >nul 2>&1
echo Da dung bridge.
