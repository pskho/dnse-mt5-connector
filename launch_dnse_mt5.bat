@echo off
setlocal
cd /d "%~dp0"

call "%~dp0start_trial.bat"

powershell.exe -NoProfile -ExecutionPolicy Bypass -Command ^
  "$candidates=@(); $candidates += Join-Path $env:ProgramFiles 'MetaTrader 5\terminal64.exe'; if (${env:ProgramFiles(x86)}) { $candidates += Join-Path ${env:ProgramFiles(x86)} 'MetaTrader 5\terminal64.exe' }; $terminal=$candidates | Where-Object { Test-Path $_ } | Select-Object -First 1; if (-not $terminal) { $terminal = Get-ChildItem -Path $env:ProgramFiles,${env:ProgramFiles(x86)} -Recurse -Filter terminal64.exe -ErrorAction SilentlyContinue | Where-Object { $_.FullName -like '*MetaTrader*' } | Select-Object -First 1 -ExpandProperty FullName }; if ($terminal) { Start-Process -FilePath $terminal } else { Write-Host 'Khong tim thay MetaTrader 5 terminal64.exe. Hay mo MT5 thu cong mot lan.' -ForegroundColor Yellow; pause }"
