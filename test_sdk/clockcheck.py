import socket, struct, time
def get_ntp_time(host='pool.ntp.org'):
    REFERENCE_TIME = 2208988800
    client = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    data = b'\x1b' + 47 * b'\0'
    client.settimeout(2)
    client.sendto(data, (host, 123))
    data, _ = client.recvfrom(1024)
    client.close()
    t = struct.unpack('!12I', data)[10] - REFERENCE_TIME
    return time.strftime('%Y-%m-%d %H:%M:%S', time.gmtime(t))

from datetime import datetime, timezone
local_utc = datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S')
print("System UTC:", local_utc)
try:
    ntp = get_ntp_time()
    print("NTP UTC:   ", ntp)
    
    # Calculate difference in seconds
    from datetime import datetime as dt
    fmt = '%Y-%m-%d %H:%M:%S'
    diff = (dt.strptime(local_utc, fmt) - dt.strptime(ntp, fmt)).total_seconds()
    print(f"Clock drift: {diff:+.0f} seconds")
    if abs(diff) > 60:
        print("WARNING: Clock is off by more than 60 seconds!")
    else:
        print("Clock looks OK")
except Exception as e:
    print("NTP error:", e)
