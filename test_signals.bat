@echo off
echo Testing Signal Pipeline
echo =======================
echo.

echo 1. Check API Health:
curl -s http://127.0.0.1:8080/health
echo.
echo.

echo 2. Send a mock MT5 Signal (BUY VN30F1M):
curl -s -X POST http://127.0.0.1:8080/signal -H "Content-Type: application/json" -d "{\"symbol\":\"VN30F1M\",\"side\":\"BUY\",\"quantity\":1,\"source\":\"MT5\"}"
echo.
echo.

echo 3. Get pending signals (Wait 1 second for queue processing):
timeout /t 1 /nobreak >nul
curl -s http://127.0.0.1:8080/signals
echo.
echo.

echo 4. Confirm a signal (Copy the signalId from above):
echo Command: curl -X POST http://127.0.0.1:8080/confirm -H "Content-Type: application/json" -d "{\"signalId\":\"YOUR_SIGNAL_ID\"}"
echo.

echo 5. Reject a signal:
echo Command: curl -X POST http://127.0.0.1:8080/reject -H "Content-Type: application/json" -d "{\"signalId\":\"YOUR_SIGNAL_ID\"}"
echo.

echo 6. Disable trading (Kill Switch):
echo Command: curl -X POST http://127.0.0.1:8080/kill-switch -H "Content-Type: application/json" -d "{\"enabled\":false}"
echo.
pause
