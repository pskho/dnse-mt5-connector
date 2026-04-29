# DNSE OpenAPI - Tài liệu Xác thực (Authentication) Chi tiết

> **⚠️ ĐỌC KỸ TRƯỚC KHI SỬA CODE XÁC THỰC ⚠️**  
> File này được tạo sau nhiều giờ debug để ghi lại toàn bộ kiến thức về cơ chế auth của DNSE API.  
> Sai bất kỳ điểm nào dưới đây sẽ trả về lỗi `OA-400: Authorization field missing, malformed or invalid`.

---

## 1. Tổng quan hai lớp xác thực

| Lớp | Tên | Dùng cho |
|-----|-----|----------|
| 1 | API Key + Signature (`x-api-key` + `x-signature`) | **Mọi** API call |
| 2 | Trading Token (`trading-token`) | Chỉ các API đặt/sửa/hủy lệnh |

---

## 2. Cách tạo Signature - QUAN TRỌNG

### 2.1 Thuật toán

```
HMAC-SHA256(api_secret, signing_string) → base64 → URL-encode → đặt vào x-signature header
```

### 2.2 Signing String (thứ tự BẮT BUỘC)

```
(request-target): {method_lowercase} {path_without_base_url}
date: {date_header_value}
nonce: {random_hex_string}
```

**Ví dụ cụ thể:**
```
(request-target): get /accounts
date: Wed, 29 Apr 2026 03:35:08 +0000
nonce: 40e22598c132020383c21e627a474353
```

### 2.3 Các bước tạo signature (Go)

```go
// Bước 1: Tạo date (UTC, định dạng +0000)
date := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 +0000")

// Bước 2: Tạo nonce (random hex 32 ký tự)
var b [16]byte
rand.Read(b[:])
nonce := hex.EncodeToString(b[:])

// Bước 3: Tạo signing string (thứ tự: request-target → date → nonce)
signingString := fmt.Sprintf(
    "(request-target): %s %s\ndate: %s\nnonce: %s",
    strings.ToLower(method), // "get", "post", "put", "delete"
    path,                    // "/accounts" (không có base URL, không có query string)
    date,
    nonce,
)

// Bước 4: HMAC-SHA256
mac := hmac.New(sha256.New, []byte(apiSecret))
mac.Write([]byte(signingString))
raw := base64.StdEncoding.EncodeToString(mac.Sum(nil))

// Bước 5: URL-encode base64 (BẮT BUỘC - đây là điểm hay bị bỏ qua)
signature := url.QueryEscape(raw)
// Ví dụ: "0GIxMK...st8=" → "0GIxMK...st8%3D"

// Bước 6: Tạo header value
sigHeader := fmt.Sprintf(
    `Signature keyId="%s",algorithm="hmac-sha256",headers="(request-target) date",signature="%s",nonce="%s"`,
    apiKey, signature, nonce,
)
```

### 2.4 Cách tương đương trong Python SDK (nguồn gốc)

```python
# python/dnse/common.py
mac = hmac.new(secret.encode("utf-8"), signing_string.encode("utf-8"), hashlib.sha256)
encoded = base64.b64encode(mac.digest()).decode("utf-8")
escaped = parse.quote(encoded, safe="")  # ← URL-encode (safe="" nghĩa là encode cả "/" và "=")
```

---

## 3. Headers HTTP - CRITICAL: Không để Go canonical hóa

### ❌ SAI - Go sẽ tự đổi `date` → `Date`, `x-signature` → `X-Signature`
```go
req.Header.Set("date", date)          // Bị đổi thành "Date"
req.Header.Set("x-signature", sig)   // Bị đổi thành "X-Signature"
```

### ✅ ĐÚNG - Dùng raw map để giữ nguyên tên header lowercase
```go
req.Header["x-api-key"]  = []string{apiKey}
req.Header["date"]       = []string{date}
req.Header["x-signature"] = []string{sigHeader}
```

> **Tại sao?** DNSE server phân biệt hoa thường khi đọc headers. Header name `Date` (hoa) và `date` (thường) là khác nhau. Python's urllib3 gửi headers nguyên bản không canonical, vì vậy Python SDK hoạt động còn Go thì không nếu dùng `Header.Set()`.

---

## 4. Header `date` - Định dạng

```
Mon, 02 Jan 2006 15:04:05 +0000
```

**Lưu ý quan trọng:**
- Timezone PHẢI là `+0000` (không phải `GMT`, không phải `UTC`, không phải `Z`)
- Phải là UTC (không phải local time)
- DNSE từ chối request có timestamp cũ hơn ~5 phút so với giờ thực → đồng hồ hệ thống phải chính xác

---

## 5. Path signing - Chỉ dùng path, không có query string

```go
func requestPathOnly(path string) string {
    if idx := strings.Index(path, "?"); idx >= 0 {
        return path[:idx]
    }
    return path
}

// Ví dụ:
// "/price/ohlc?symbol=VN30F1M&type=DERIVATIVE" → sign với "/price/ohlc"
// "/accounts/0001007412/orders" → sign với "/accounts/0001007412/orders"
```

---

## 6. Header x-signature - Định dạng đầy đủ

```
Signature keyId="{api_key}",algorithm="hmac-sha256",headers="(request-target) date",signature="{url_encoded_base64}",nonce="{hex_32_chars}"
```

**Lưu ý:**
- Không có dấu cách sau dấu phẩy (`,`)
- `headers` field LUÔN là `"(request-target) date"` dù signing string có nonce
- `nonce` trong header chỉ dùng để server verify, không ảnh hưởng `headers` field

---

## 7. Lớp bảo mật thứ 2: Trading Token

Dùng cho các API đặt lệnh (POST `/accounts/orders`, PUT, DELETE).

```go
// Thêm vào header sau khi có trading token
req.Header.Set("trading-token", tradingToken)
```

### Cách lấy Trading Token

```
POST /registration/trading-token
Body: {"otpType": "email_otp", "passcode": "123456"}
```

- Trading Token có hiệu lực **8 giờ**
- Tài khoản chỉ dùng được 1 phương thức OTP: `email_otp` hoặc `smart_otp`
- Truyền sai phương thức → request bị từ chối

---

## 8. Các lỗi thường gặp và cách xử lý

### Lỗi `OA-400: Authorization field missing, malformed or invalid`

Nguyên nhân theo thứ tự ưu tiên debug:

1. **Header name bị canonical hóa bởi Go** → Dùng `req.Header["x-signature"]` thay vì `req.Header.Set()`
2. **Signature không URL-encoded** → Phải dùng `url.QueryEscape(raw)` trên base64
3. **Timestamp cũ hơn 5 phút** → Đồng hồ hệ thống lệch hoặc đang dùng timestamp hardcode
4. **Path có query string** → Chỉ sign phần path, cắt bỏ `?...`
5. **Method không lowercase** → Signing string phải dùng `"get"`, không phải `"GET"`

### Lỗi `OA-401: Unauthorized`

- Trading token hết hạn (8 giờ)
- Trading token sai phương thức OTP

---

## 9. Mock mode

Trong `config/config.yaml`:
```yaml
dnse:
  mock: false  # false = gọi API thật, true = dữ liệu giả (giá 1200 cho mọi nến)

market_data:
  mock: false  # false = WebSocket thật, true = dữ liệu sine wave giả
```

> Khi `mock: true`, mọi API call đều bypass, không gọi DNSE server, dữ liệu OHLC hardcode là `1200.0`.

---

## 10. File implementation chính

| File | Mục đích |
|------|----------|
| `internal/api/dnse_client.go` | DNSE API client, `GenerateSignature()`, `send()`, `fetchTradingToken()` |
| `internal/marketdata/publisher.go` | WebSocket market data stream, bridge to MT5 |
| `internal/marketdata/history.go` | Lấy dữ liệu OHLC lịch sử |
| `config/config.yaml` | Cấu hình API key, secret, mock mode |

---

## 11. Code tham chiếu chuẩn (đã verified)

Hàm `GenerateSignature` đúng hoàn toàn (test status 200):

```go
func GenerateSignature(apiKey, secret, method, path, date string) string {
    nonce := generateNonce() // random 32-char hex

    // Signing string: thứ tự cố định (request-target) → date → nonce
    signingString := fmt.Sprintf(
        "(request-target): %s %s\ndate: %s\nnonce: %s",
        strings.ToLower(method), path, date, nonce,
    )

    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(signingString))
    raw := base64.StdEncoding.EncodeToString(mac.Sum(nil))

    // PHẢI URL-encode base64 (/ → %2F, + → %2B, = → %3D)
    signature := url.QueryEscape(raw)

    return fmt.Sprintf(
        `Signature keyId="%s",algorithm="hmac-sha256",headers="(request-target) date",signature="%s",nonce="%s"`,
        apiKey, signature, nonce,
    )
}

// Gửi request - PHẢI dùng raw map, không dùng Header.Set()
req.Header["x-api-key"]   = []string{apiKey}
req.Header["date"]        = []string{date}
req.Header["x-signature"] = []string{sigHeader}
```

---

## 12. SDK tham chiếu chính thức

- **Python SDK**: https://github.com/dnse-tech/openapi-sdk/tree/main/python
  - `python/dnse/client.py` → cách gửi request
  - `python/dnse/common.py` → `build_signature()` function
- **Docs chính thức**: https://developers.dnse.com.vn/docs/guide/intro/authentication
