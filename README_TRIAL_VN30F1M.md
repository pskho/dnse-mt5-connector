# DNSE MT5 Connector - Bản dùng thử VN30F1M

Bản này được đóng gói để khách hàng xem giá `VN30F1M` trên MT5 trước.

## Mục tiêu
- Cài nhanh vào MT5
- Chạy local trên Windows
- Xem được realtime `VN30F1M`
- Không cần build Go hay C++ lại

## Các bước nhanh nhất

### 1. Điền DNSE API key
Mở file:

`config\config.yaml`

Tìm 2 dòng:

```yaml
api_key: "PASTE_DNSE_API_KEY_HERE"
api_secret: "PASTE_DNSE_API_SECRET_HERE"
```

và thay bằng thông tin DNSE thực tế.

### 2. Chạy bridge
Chạy file:

`start_trial.bat`

Bridge sẽ mở local tại:

`http://127.0.0.1:8080/setup`

### 3. Cài vào MT5
Chạy file:

`deploy_mt5.bat`

Script sẽ:
- copy `DNSEBridge.dll`
- copy `DNSE_MarketData_Bridge.mq5`
- xóa bản EA cũ bị trùng ở thư mục root `Experts`
- nếu tìm thấy MetaEditor sẽ tự compile luôn

### 4. Trong MT5
- Mở `Tools -> Options -> Expert Advisors`
- Bật `Allow DLL imports`
- Vào `Navigator -> Expert Advisors -> DNSE`
- Gắn `DNSE_MarketData_Bridge` vào chart

### 5. Kiểm tra giá
Chart custom symbol:

`VN30F1M_DNSE`

Nếu bridge đang chạy đúng, chart sẽ cập nhật giá `VN30F1M`.

## Nếu Windows hiện cảnh báo
Trong một số máy, Windows SmartScreen có thể cảnh báo khi chạy `bridge.exe`.

Khi đó:
1. Chọn `More info`
2. Chọn `Run anyway`

Đây là bản dùng thử nội bộ, chưa ký số code signing.

## Các file cho người dùng
- `start_trial.bat`: chạy bridge
- `stop_trial.bat`: dừng bridge
- `open_setup.bat`: mở trang setup nếu bridge đang chạy
- `deploy_mt5.bat`: cài vào MT5

## Ghi chú
- Bản dùng thử này ưu tiên xem giá `VN30F1M`
- Trading và OTP chưa phải mục tiêu chính của gói demo này
- Nếu muốn đồng bộ history, vào dashboard sau khi bridge chạy
