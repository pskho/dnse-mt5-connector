@echo off
setlocal
cd /d "%~dp0"

if exist bridge.pid (
    set /p EXISTING_PID=<bridge.pid
    tasklist /FI "PID eq %EXISTING_PID%" | find "%EXISTING_PID%" >nul 2>&1
    if errorlevel 1 (
        echo Tim thay bridge.pid cu, se xoa va khoi dong lai.
        del /q bridge.pid >nul 2>&1
    ) else (
        echo Bridge dang chay voi PID %EXISTING_PID%.
        echo Neu muon chay lai, hay dung stop_trial.bat truoc.
        start "" http://127.0.0.1:8080/setup
        exit /b 0
    )
)

echo Khoi dong DNSE MT5 Connector...
powershell.exe -NoProfile -ExecutionPolicy Bypass -Command ^
  "try { $p = Start-Process -FilePath '.\bridge.exe' -WorkingDirectory '%~dp0' -PassThru; Set-Content -Path '.\bridge.pid' -Value $p.Id; Start-Sleep -Seconds 2; Start-Process 'http://127.0.0.1:8080/setup' } catch { Write-Host ''; Write-Host 'Khong khoi dong duoc bridge.exe.' -ForegroundColor Red; Write-Host 'Neu Windows Security/SmartScreen hien canh bao, hay chon More info -> Run anyway hoac Allow on device.' -ForegroundColor Yellow; Write-Host 'Neu cong 8080 dang bi ung dung khac dung, hay tat ung dung do roi chay lai.' -ForegroundColor Yellow; Write-Host ''; throw }"

if errorlevel 1 (
    echo.
    echo Khoi dong that bai.
    echo 1. Kiem tra Windows Security co chan bridge.exe khong.
    echo 2. Kiem tra cong 8080 co dang bi ung dung khac su dung khong.
    echo 3. Chay lai file nay sau khi da xu ly.
    pause
    exit /b 1
)

echo Da gui lenh khoi dong bridge.
