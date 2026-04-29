@echo off
setlocal
cd /d "%~dp0"

if exist bridge.pid (
    echo Dang co file bridge.pid. Neu bridge khong con chay, hay chay stop_trial.bat truoc.
)

echo Khoi dong DNSE MT5 Connector...
powershell.exe -NoProfile -ExecutionPolicy Bypass -Command ^
  "$p = Start-Process -FilePath '.\bridge.exe' -WorkingDirectory '%~dp0' -PassThru -WindowStyle Hidden; Set-Content -Path '.\bridge.pid' -Value $p.Id; Start-Sleep -Seconds 2; Start-Process 'http://127.0.0.1:8080/setup'"
echo Da gui lenh khoi dong. Neu trinh duyet khong tu mo, hay chay open_setup.bat
