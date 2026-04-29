import hmac, hashlib, base64
from urllib import parse

secret = "j5x4a-oTlfa6NtWdoclIp6wi-xLC2cN-0pWfU0I3tdJUKvMC_DuAsQlANmyZO3eyGKPcX2UK-fg29KION1m8Bg"

# Exact same inputs as Go
method = "get"
path = "/accounts"
date = "Wed, 29 Apr 2026 03:32:00 +0000"
nonce = "18aab65ea5f18ba0"

signing_string = f"(request-target): {method} {path}\ndate: {date}\nnonce: {nonce}"
print("signing_string repr:", repr(signing_string))
print()

mac = hmac.new(secret.encode("utf-8"), signing_string.encode("utf-8"), hashlib.sha256)
raw = base64.b64encode(mac.digest()).decode("utf-8")
escaped = parse.quote(raw, safe="")
print("Python raw b64:", raw)
print("Python escaped:", escaped)
