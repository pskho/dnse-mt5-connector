#!/usr/bin/env python3
import base64
import hashlib
import hmac
import json
import os
from datetime import datetime, timezone
from urllib import parse, request
from uuid import uuid4


def get_date_header_name():
    return os.getenv("DATE_HEADER", "Date")


def build_signature(secret, method, path, date_value, algorithm, nonce=None, header_name=None):
    header_name = header_name or get_date_header_name()
    header_key = header_name.lower()
    headers = f"(request-target) {header_key}"
    signature_string = (
        f"(request-target): {method.lower()} {path}\n" f"{header_key}: {date_value}"
    )
    if nonce:
        signature_string += f"\nnonce: {nonce}"

    if algorithm == "hmac-sha256":
        digestmod = hashlib.sha256
    elif algorithm == "hmac-sha384":
        digestmod = hashlib.sha384
    elif algorithm == "hmac-sha512":
        digestmod = hashlib.sha512
    else:
        digestmod = hashlib.sha1

    mac = hmac.new(secret.encode("utf-8"), signature_string.encode("utf-8"), digestmod)
    encoded = base64.b64encode(mac.digest()).decode("utf-8")
    escaped = parse.quote(encoded, safe="")

    return headers, escaped
