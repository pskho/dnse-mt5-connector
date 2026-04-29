import urllib3
from common import get_date_header_name, build_signature
from datetime import datetime, timezone
import uuid

api_key = "eyJvcmciOiJkbnNlIiwiaWQiOiI3YTdmYjRiNWM0ZGM0ODhiOWQ0ZmJmOWZmZjE0YTllMiIsImgiOiJtdXJtdXIxMjgifQ=="
api_secret = "j5x4a-oTlfa6NtWdoclIp6wi-xLC2cN-0pWfU0I3tdJUKvMC_DuAsQlANmyZO3eyGKPcX2UK-fg29KION1m8Bg"

# Use exact values from Go
date_value = "Wed, 29 Apr 2026 03:32:00 +0000"
date_header_name = "Date"
nonce = "18aab65ea5f18ba0"

headers_list, signature = build_signature(
    api_secret,
    "GET",
    "/accounts",
    date_value,
    "hmac-sha256",
    nonce=nonce,
    header_name=date_header_name,
)

sig_header = (
    f'Signature keyId="{api_key}",algorithm="hmac-sha256",'
    f'headers="{headers_list}",signature="{signature}"'
)
if nonce:
    sig_header += f',nonce="{nonce}"'

print("Python x-signature:", sig_header)
print()

http = urllib3.PoolManager()
resp = http.request("GET", "https://openapi.dnse.com.vn/accounts", headers={
    date_header_name: date_value,
    "X-Signature": sig_header,
    "x-api-key": api_key,
})
print("Status:", resp.status)
