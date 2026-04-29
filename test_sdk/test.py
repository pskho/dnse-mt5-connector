import urllib3
import json
from common import get_date_header_name, build_signature
from datetime import datetime, timezone
import uuid

api_key = "eyJvcmciOiJkbnNlIiwiaWQiOiI3YTdmYjRiNWM0ZGM0ODhiOWQ0ZmJmOWZmZjE0YTllMiIsImgiOiJtdXJtdXIxMjgifQ=="
api_secret = "j5x4a-oTlfa6NtWdoclIp6wi-xLC2cN-0pWfU0I3tdJUKvMC_DuAsQlANmyZO3eyGKPcX2UK-fg29KION1m8Bg"

method = "GET"
path = "/accounts"
url = "https://openapi.dnse.com.vn/accounts"

date_value = "Wed, 29 Apr 2026 00:00:00 +0000"
date_header_name = get_date_header_name()
nonce = "1234567890abcdef"

headers_list, signature = build_signature(
    api_secret,
    method,
    path,
    date_value,
    "hmac-sha256",
    nonce=nonce,
    header_name=date_header_name,
)

signature_header_value = (
    f'Signature keyId="{api_key}",algorithm="hmac-sha256",'
    f'headers="{headers_list}",signature="{signature}"'
)
if nonce:
    signature_header_value += f',nonce="{nonce}"'

req_headers = {
    date_header_name: date_value,
    "X-Signature": signature_header_value,
    "x-api-key": api_key,
}

http = urllib3.PoolManager()
resp = http.request(method, url, headers=req_headers)
print(req_headers)
