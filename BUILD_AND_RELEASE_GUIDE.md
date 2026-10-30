# Hướng dẫn build, test và phát hành DNSE MT5 Connector

Tài liệu này gom toàn bộ các bước build, đóng gói, kiểm tra local và phát hành để có thể dùng lại nhiều lần mà không cần hỏi lại.

## 1. Vị trí thư mục dự án

```powershell
cd "C:\Users\Admin\Documents\Codex\2026-04-28\you-are-a-senior-software-engineer"
```

---

## 2. Các file chạy nhanh

### Build + publish đầy đủ

File:

```text
release_one_click.bat
```

Công dụng:
- build `bridge.exe`
- build `DNSEBridge.dll`
- deploy vào MT5
- auto-compile EA nếu có MetaEditor
- tạo file `.zip`
- tạo file cài `.exe`
- commit source cần thiết
- push lên git
- tạo hoặc cập nhật GitHub Release

Chỉ cần bấm đúp:

```text
release_one_click.bat
```

### Build local để test, không đẩy git, không đụng release

File:

```text
release_one_click_test.bat
```

Công dụng:
- build toàn bộ local
- không push git
- không upload GitHub Release

Chỉ cần bấm đúp:

```text
release_one_click_test.bat
```

---

## 3. Script chính

### Script phát hành đầy đủ

File:

```text
release_one_click.ps1
```

Chạy tay:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\release_one_click.ps1
```

### Chạy local nhưng bỏ qua git và release

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\release_one_click.ps1 -SkipGit -SkipRelease
```

### Chạy nhưng không deploy vào MT5 local

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\release_one_click.ps1 -SkipDeployMt5
```

### Chạy nhưng không push git

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\release_one_click.ps1 -SkipGit
```

### Chạy nhưng không cập nhật GitHub Release

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\release_one_click.ps1 -SkipRelease
```

---

## 4. Bộ lệnh rebuild thủ công chuẩn

Nếu cần rebuild từng bước bằng tay:

```powershell
cd "C:\Users\Admin\Documents\Codex\2026-04-28\you-are-a-senior-software-engineer"

$env:GOCACHE = Join-Path (Get-Location) ".gocache"

if (Test-Path .\bridge.exe) { Remove-Item .\bridge.exe -Force }
if (Test-Path .\cpp\build) { Remove-Item .\cpp\build -Recurse -Force }
if (Test-Path .\dist) { Remove-Item .\dist -Recurse -Force }

powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\build_bridge.ps1 -Output .\bridge.exe
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\build.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\deploy_mt5.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\package_trial.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\build_installer.ps1 -InstallerName "DNSE-MT5-Connector-VN30F1M-Setup.exe"
```

---

## 5. Kiểm tra nhanh sau khi build

```powershell
Get-Item .\bridge.exe | Select-Object FullName,Length,LastWriteTime
Get-Item .\cpp\build\Release\DNSEBridge.dll | Select-Object FullName,Length,LastWriteTime
Get-ChildItem .\dist
```

Chạy thử bridge:

```powershell
.\bridge.exe
```

Khi đúng, bridge phải chạy như server, không được in log test kiểu:

```text
date:
nonce:
x-signature:
Status: 200
Body: ...
```

---

## 6. Các file đầu ra quan trọng

### File zip cho khách

```text
dist\DNSE-MT5-Connector-VN30F1M-Trial.zip
```

### File cài đặt exe cho khách

```text
dist\DNSE-MT5-Connector-VN30F1M-Setup.exe
```

### File bridge server

```text
bridge.exe
```

### DLL cho MT5

```text
cpp\build\Release\DNSEBridge.dll
```

### EA MQL5

```text
mql5\DNSE_MarketData_Bridge.mq5
```

---

## 7. Cấu hình các mã mặc định hiện tại

Bộ mã mặc định đang dùng:

- `VN30F1M`
- `VN30F2M`
- `VN30F1Q`
- `VN30F2Q`
- `V100F1M`
- `V100F2M`
- `V100F1Q`
- `V100F2Q`
- `VN30`
- `VN100`

Lưu ý:
- với phái sinh VN100, chuẩn đúng là `V100F*`
- hệ đã tự chuẩn hóa nếu còn sót cấu hình cũ dạng `VN100F*`

---

## 8. Phase B hiện tại

Backend đã hỗ trợ:
- nhiều symbol thật sự ở phía bridge
- handshake subscribe theo symbol
- history snapshot theo từng symbol
- Setup/Cấu hình chọn danh sách mã và mã chính

MT5 hiện dùng theo hướng:
- mỗi chart / EA gắn với một symbol nguồn
- bridge có khả năng phục vụ nhiều symbol đúng nghĩa hơn trước

---

## 9. Upload lại release thủ công nếu cần

Nếu đã build xong và chỉ muốn cập nhật file lên GitHub Release:

```powershell
gh release upload v0.1.0-trial ".\dist\DNSE-MT5-Connector-VN30F1M-Trial.zip" ".\dist\DNSE-MT5-Connector-VN30F1M-Setup.exe" --clobber --repo "pskho/dnse-mt5-connector"
```

---

## 10. Khi chỉ muốn push source code sạch

Không đẩy `dist`, log, data hay config nhạy cảm.

Script `release_one_click.ps1` đã tự stage chọn lọc các file source cần thiết.

Nếu tự làm tay:

```powershell
git add `
  .gitignore `
  README.md `
  README_TRIAL_VN30F1M.md `
  RELEASE_NOTES_v0.1.0-trial.md `
  build.ps1 `
  build_bridge.ps1 `
  build_installer.ps1 `
  cmd `
  config\config.yaml.example `
  cpp\CMakeLists.txt `
  cpp\DNSEBridge.cpp `
  deploy_mt5.bat `
  deploy_mt5.ps1 `
  docs `
  go.mod `
  go.sum `
  install_trial.ps1 `
  installer_bootstrap.cs `
  internal `
  mql5\DNSE_MarketData_Bridge.mq5 `
  open_setup.bat `
  package_trial.ps1 `
  publish_release.ps1 `
  push_git.ps1 `
  release_one_click.ps1 `
  release_one_click.bat `
  release_one_click_test.bat `
  run_bridge_console.bat `
  start_trial.bat `
  stop_trial.bat `
  test_signals.bat
```

---

## 11. Nếu bridge hoặc installer chạy lỗi trên máy khách

### Nếu Windows cảnh báo khi chạy

Hướng dẫn khách:
- bấm `More info`
- bấm `Run anyway`

### Nếu bridge mở rồi tắt ngay

Chạy:

```text
run_bridge_console.bat
```

### Nếu `/setup` báo `not found`

Khả năng cao:
- đang dùng bản `bridge.exe` cũ
- hoặc gói phát hành chưa được build lại từ source mới nhất

Khi đó cần build lại theo mục 4 hoặc dùng `release_one_click_test.bat`.

---

## 12. Khuyến nghị vận hành

- Dùng `release_one_click_test.bat` trước để kiểm tra local.
- Khi đã ổn mới dùng `release_one_click.bat` để phát hành.
- Ưu tiên gửi khách bản `.zip` nếu máy khách hay bị Windows chặn file `.exe`.
- Không đưa `config/config.yaml` thật lên git hoặc vào gói gửi khách.

---

## 13. Các file tiện dùng nhất

- Hướng dẫn này:
  - `BUILD_AND_RELEASE_GUIDE.md`
- Build và phát hành đầy đủ:
  - `release_one_click.bat`
- Build local an toàn:
  - `release_one_click_test.bat`
- Script phát hành chính:
  - `release_one_click.ps1`

---

## 14. Lenh build 1 click nen dung

Build local an toan nhat, khong deploy MT5, khong commit/push, khong tao GitHub Release:

```text
build_one_click.bat
```

Lenh PowerShell tuong duong:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\release_one_click.ps1 -SkipGit -SkipRelease -SkipDeployMt5
```

Build local va deploy EA/DLL vao MT5 neu may co MT5:

```text
release_one_click_test.bat
```

Phat hanh day du len GitHub Release:

```text
release_one_click.bat
```
