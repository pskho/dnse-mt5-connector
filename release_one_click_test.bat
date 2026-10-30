@echo off
setlocal
cd /d "%~dp0"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0release_one_click.ps1" -SkipGit -SkipRelease
if errorlevel 1 (
  echo.
  echo Co loi trong qua trinh build local. Xem thong bao o tren de xu ly.
  pause
  exit /b 1
)
echo.
echo Da chay xong ban test local (khong push git, khong tao release).
pause
