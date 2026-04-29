# DNSE MT5 Connector v0.1.0-trial

Bản dùng thử đầu tiên để khách hàng xem giá `VN30F1M` trên MetaTrader 5.

## Tệp nên tải
- **Khuyến nghị:** `DNSE-MT5-Connector-VN30F1M-Trial.zip`

Bản `.zip` là lựa chọn ổn định nhất cho giai đoạn dùng thử hiện tại.

## Điểm chính
- Bridge local Windows kết nối DNSE và MT5
- Realtime market data cho `VN30F1M`
- Đồng bộ dữ liệu lịch sử để nạp vào custom symbol MT5
- Tự động copy DLL và EA vào MT5
- Có script hỗ trợ cài nhanh vào MT5

## Phạm vi bản này
- Ưu tiên xem giá `VN30F1M`
- Chưa coi đây là bản trading production cho khách hàng cuối
- Trading, OTP và các tính năng nặng vẫn có trong source, nhưng bản release này tập trung vào market data demo

## Cách dùng cho khách hàng
1. Tải file `DNSE-MT5-Connector-VN30F1M-Trial.zip`
2. Giải nén
3. Mở file `config\config.yaml` và điền `DNSE API key` / `DNSE API secret`
4. Chạy `start_trial.bat`
5. Chạy `deploy_mt5.bat`
6. Mở MT5 và gắn EA `DNSE_MarketData_Bridge`
7. Xem chart `VN30F1M_DNSE`

## Lưu ý khi chạy trên Windows
- Trên một số máy, Windows SmartScreen có thể cảnh báo khi chạy `bridge.exe`
- Nếu xuất hiện cảnh báo, vui lòng chọn:
  - `More info`
  - `Run anyway`
- Đây là bản dùng thử nội bộ, chưa ký số code signing

## Ghi chú
- Nếu MT5 chưa bật `Allow DLL imports`, cần bật trong `Tools -> Options -> Expert Advisors`
- Bản này phù hợp để demo và thử nghiệm nội bộ
- Nếu Windows cảnh báo với file `.exe`, hãy dùng bản `.zip`
- Nếu `http://127.0.0.1:8080/setup` báo `not found`, hãy dừng bridge và chạy lại `start_trial.bat`
