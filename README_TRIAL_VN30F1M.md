# DNSE MT5 Connector - Ban dung thu VN30F1M

Ban nay duoc dong goi de khach hang xem gia `VN30F1M` tren MT5 truoc.

## Muc tieu
- Cai nhanh vao MT5
- Chay local tren Windows
- Xem duoc realtime `VN30F1M`
- Khong can build Go hay C++ lai

## Cac buoc nhanh nhat

### 1. Dien DNSE API key
Mo file:

`config\config.yaml`

Tim 2 dong:

```yaml
api_key: "PASTE_DNSE_API_KEY_HERE"
api_secret: "PASTE_DNSE_API_SECRET_HERE"
```

va thay bang thong tin DNSE thuc te.

### 2. Chay bridge
Chay file:

`start_trial.bat`

Bridge se mo local tai:

`http://127.0.0.1:8080/setup`

### 3. Cai vao MT5
Chay file:

`deploy_mt5.bat`

Script se:
- copy `DNSEBridge.dll`
- copy `DNSE_MarketData_Bridge.mq5`
- xoa ban EA cu bi trung o thu muc root `Experts`
- neu tim thay MetaEditor se tu compile luon

### 4. Trong MT5
- Mo `Tools -> Options -> Expert Advisors`
- Bat `Allow DLL imports`
- Vao `Navigator -> Expert Advisors -> DNSE`
- Gan `DNSE_MarketData_Bridge` vao chart

### 5. Kiem tra gia
Chart custom symbol:

`VN30F1M_DNSE`

Neu bridge dang chay dung, chart se cap nhat gia `VN30F1M`.

## Cac file cho nguoi dung
- `start_trial.bat`: chay bridge
- `stop_trial.bat`: dung bridge
- `open_setup.bat`: mo trang setup neu bridge dang chay
- `deploy_mt5.bat`: cai vao MT5

## Ghi chu
- Ban dung thu nay uu tien xem gia `VN30F1M`
- Trading va OTP chua phai muc tieu chinh cua goi demo nay
- Neu muon dong bo history, vao dashboard sau khi bridge chay
