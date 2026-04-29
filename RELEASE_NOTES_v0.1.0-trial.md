# DNSE MT5 Connector v0.1.0-trial

Ban dung thu dau tien de khach hang xem gia `VN30F1M` tren MetaTrader 5.

## Diem chinh
- Bridge local Windows ket noi DNSE va MT5
- Realtime market data cho `VN30F1M`
- Dong bo lich su de nap vao custom symbol MT5
- Tu dong copy DLL va EA vao MT5
- File cai dat `.exe` de khach hang dung thu nhanh

## Pham vi ban nay
- Uu tien xem gia `VN30F1M`
- Chua coi day la ban trading production cho khach hang cuoi
- Trading, OTP va cac tinh nang nang duoc giu trong source, nhung ban release nay tap trung vao market data demo

## Cach dung cho khach hang
1. Tai file `DNSE-MT5-Connector-VN30F1M-Setup.exe`
2. Chay installer
3. Dien `DNSE API key` va `DNSE API secret`
4. Mo MT5 va gan EA `DNSE_MarketData_Bridge`
5. Xem chart `VN30F1M_DNSE`

## Ghi chu
- Neu MT5 chua bat `Allow DLL imports`, can bat trong `Tools -> Options -> Expert Advisors`
- Ban nay phu hop de demo va thu nghiem noi bo
