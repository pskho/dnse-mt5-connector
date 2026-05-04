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
  "try { $stdout = Join-Path '%~dp0' 'bridge_out.txt'; $stderr = Join-Path '%~dp0' 'bridge_err.txt'; Remove-Item $stdout,$stderr -Force -ErrorAction SilentlyContinue; $p = Start-Process -FilePath '.\bridge.exe' -WorkingDirectory '%~dp0' -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru; Start-Sleep -Seconds 3; if ($p.HasExited) { Write-Host ''; Write-Host 'bridge.exe da thoat ngay sau khi chay.' -ForegroundColor Red; if (Test-Path $stdout) { Write-Host ''; Write-Host '--- bridge_out.txt ---' -ForegroundColor Yellow; Get-Content $stdout }; if (Test-Path $stderr) { Write-Host ''; Write-Host '--- bridge_err.txt ---' -ForegroundColor Yellow; Get-Content $stderr }; exit 1 }; Set-Content -Path '.\bridge.pid' -Value $p.Id; Start-Process 'http://127.0.0.1:8080/setup' } catch { Write-Host ''; Write-Host 'Khong khoi dong duoc bridge.exe.' -ForegroundColor Red; Write-Host 'Neu Windows Security/SmartScreen hien canh bao, hay chon More info -> Run anyway hoac Allow on device.' -ForegroundColor Yellow; Write-Host 'Neu cong 8080 dang bi ung dung khac dung, hay tat ung dung do roi chay lai.' -ForegroundColor Yellow; Write-Host 'Co the mo file run_bridge_console.bat de xem loi truc tiep.' -ForegroundColor Yellow; Write-Host ''; throw }"

if errorlevel 1 (
    echo.
    echo Khoi dong that bai.
    echo 1. Kiem tra Windows Security co chan bridge.exe khong.
    echo 2. Kiem tra cong 8080 co dang bi ung dung khac su dung khong.
    echo 3. Kiem tra bridge_out.txt va bridge_err.txt trong thu muc cai dat.
    echo 4. Hoac chay run_bridge_console.bat de xem loi truc tiep.
    pause
    exit /b 1
)

echo Da gui lenh khoi dong bridge.
