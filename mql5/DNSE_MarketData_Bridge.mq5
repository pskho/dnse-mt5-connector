//+------------------------------------------------------------------+
//| DNSE MT5 Market Data Bridge                                      |
//| Updates custom symbol VN30F1M_DNSE from DNSEBridge.dll           |
//+------------------------------------------------------------------+
#property strict
#property version   "1.11"

struct MqlRateLite {
   long     time;
   double   open;
   double   high;
   double   low;
   double   close;
   long     tick_volume;
};

#import "DNSEBridge.dll"
bool ConnectBridge(string endpoint);
bool IsConnected();
bool IsHistoryReady();
bool GetLatestTick(double &bid, double &ask, double &last, long &volume, long &timestamp_ms);
void DisconnectBridge();
int GetHistoricalRates(MqlRateLite &buffer[], int max_count);
int GetHistoricalRatesCount();
int GetHistoricalRatesRange(int offset, MqlRateLite &buffer[], int max_count);
void ClearHistoricalRates();
#import

input string InpEndpoint      = "127.0.0.1:9090";
input string InpSourceSymbol  = "VN30F1M";
input string InpCustomSymbol  = "VN30F1M_DNSE";
input string InpHistoryMarketType = "DERIVATIVE";
input int    InpHistoryResolution = 1;
input int    InpTimerMs       = 100;
input int    InpReconnectSec  = 3;
input int    InpHistoryTimeoutMs = 30000;
input int    InpAutoRecentHistoryDays = 0;   // 0=manual only, 1=today, 7=last week
input int    InpAutoRecentSyncDelaySec = 15; // wait for realtime before any history backfill
input bool   InpEnableSignalBridge = false;

long     g_last_timestamp_ms = 0;
double   g_last_bid          = 0.0;
double   g_last_ask          = 0.0;
double   g_last_last         = 0.0;
long     g_last_volume       = 0;
datetime g_last_reconnect    = 0;
bool     g_warned_disconnect = false;
datetime g_last_signal_time  = 0;
bool     g_first_tick_seen   = false;
bool     g_recent_history_done = false;
datetime g_recent_history_not_before = 0;
datetime g_recent_history_retry_after = 0;
datetime g_last_realtime_log = 0;

struct PendingSignal {
   string id;
   datetime time;
};
PendingSignal g_pending_signals[];

void TrackSignal(string signalId)
{
   int size = ArraySize(g_pending_signals);
   ArrayResize(g_pending_signals, size + 1);
   g_pending_signals[size].id = signalId;
   g_pending_signals[size].time = TimeCurrent();
}

void PollPendingSignals()
{
   if(!InpEnableSignalBridge)
      return;
   datetime now = TimeCurrent();
   for(int i = ArraySize(g_pending_signals) - 1; i >= 0; i--)
   {
      if(now - g_pending_signals[i].time > 60)
      {
         PrintFormat("DNSE bridge: Signal %s timed out waiting for confirmation.", g_pending_signals[i].id);
         ArrayRemove(g_pending_signals, i, 1);
         continue;
      }
      
      string sigId = g_pending_signals[i].id;
      string url = "http://127.0.0.1:8080/order/client/signal-" + sigId;
      
      char data[];
      char result[];
      string result_headers;
      int res = WebRequest("GET", url, "", 500, data, result, result_headers);
      
      if(res == 200)
      {
         string response = CharArrayToString(result, 0, WHOLE_ARRAY, CP_UTF8);
         int oStart = StringFind(response, "\"orderId\":\"") + 11;
         int oEnd = StringFind(response, "\"", oStart);
         string orderId = "";
         if(oStart >= 11 && oEnd > oStart) orderId = StringSubstr(response, oStart, oEnd - oStart);
         
         int sStart = StringFind(response, "\"status\":\"") + 10;
         int sEnd = StringFind(response, "\"", sStart);
         string status = "";
         if(sStart >= 10 && sEnd > sStart) status = StringSubstr(response, sStart, sEnd - sStart);
         
         PrintFormat("DNSE bridge: Order confirmed for signal! orderId=%s, status=%s", orderId, status);
         ArrayRemove(g_pending_signals, i, 1);
      }
   }
}

void SendSignal(string side)
{
   string url = "http://127.0.0.1:8080/signal";
   string payload = StringFormat("{\"symbol\":\"%s\",\"side\":\"%s\",\"quantity\":1,\"source\":\"MT5\"}", InpSourceSymbol, side);
   
   char data[];
   StringToCharArray(payload, data, 0, StringLen(payload), CP_UTF8);
   
   char result[];
   string result_headers;
   string headers = "Content-Type: application/json\r\n";
   
   int res = WebRequest("POST", url, headers, 1000, data, result, result_headers);
   
   if(res == 200)
   {
      string response = CharArrayToString(result, 0, WHOLE_ARRAY, CP_UTF8);
      PrintFormat("DNSE bridge: Signal %s sent. Response: %s", side, response);
      
      int start = StringFind(response, "\"signalId\":\"") + 12;
      int end = StringFind(response, "\"", start);
      if(start >= 12 && end > start)
      {
         string signalId = StringSubstr(response, start, end - start);
         TrackSignal(signalId);
      }
   }
   else
   {
      string errorMsg = CharArrayToString(result, 0, WHOLE_ARRAY, CP_UTF8);
      PrintFormat("DNSE bridge: Failed to send %s signal. Code: %d, Msg: %s", side, res, errorMsg);
   }
}

void CheckForSignal()
{
   if(!InpEnableSignalBridge)
      return;
   MqlRates rates[];
   if(CopyRates(InpCustomSymbol, PERIOD_M1, 0, 1, rates) <= 0)
      return;
      
   datetime candle_time = rates[0].time;
   if(candle_time <= g_last_signal_time)
      return; // Already sent signal for this candle
      
   double open = rates[0].open;
   double close = rates[0].close;
   
   string side = "";
   if(close > open)
      side = "BUY";
   else if(close < open)
      side = "SELL";
      
   if(side != "")
   {
      g_last_signal_time = candle_time;
      SendSignal(side);
   }
}

bool EnsureCustomSymbol()
{
   if(!SymbolSelect(InpCustomSymbol, true) && !CustomSymbolCreate(InpCustomSymbol, "DNSE"))
   {
      int err = GetLastError();
      PrintFormat("DNSE bridge: cannot create custom symbol %s, error=%d", InpCustomSymbol, err);
      ResetLastError();
      return false;
   }

   CustomSymbolSetInteger(InpCustomSymbol, SYMBOL_DIGITS, 1);
   CustomSymbolSetDouble(InpCustomSymbol, SYMBOL_POINT, 0.1);
   CustomSymbolSetString(InpCustomSymbol, SYMBOL_DESCRIPTION, "DNSE realtime " + InpSourceSymbol);
   SymbolSelect(InpCustomSymbol, true);
   return true;
}

string SafeCharArrayToString(char &buffer[])
{
   if(ArraySize(buffer) <= 0)
      return "";
   return CharArrayToString(buffer, 0, WHOLE_ARRAY, CP_UTF8);
}

int ParseCandlesSynced(string response)
{
   int start = StringFind(response, "\"candlesSynced\":");
   if(start < 0)
      return -1;
   start += 16;
   while(start < StringLen(response))
   {
      ushort ch = StringGetCharacter(response, start);
      if(ch >= '0' && ch <= '9')
         break;
      start++;
   }
   if(start >= StringLen(response))
      return -1;
   string number = "";
   while(start < StringLen(response))
   {
      ushort ch = StringGetCharacter(response, start);
      if(ch < '0' || ch > '9')
         break;
      number += StringSubstr(response, start, 1);
      start++;
   }
   return (int)StringToInteger(number);
}

bool RequestHistory(bool forceFull, long firstTimeMs, long lastTimeMs, int lookbackDays, int &candlesSynced)
{
   string url;
   string payload;
   candlesSynced = -1;
   if(forceFull)
   {
      url = "http://127.0.0.1:8080/history/full";
      payload = StringFormat("{\"lookbackDays\":%d,\"symbol\":\"%s\",\"marketType\":\"%s\",\"resolution\":%d}", lookbackDays, InpSourceSymbol, InpHistoryMarketType, InpHistoryResolution);
      PrintFormat("DNSE bridge: Triggering history backfill for %d day(s)...", lookbackDays);
   }
   else
   {
      url = "http://127.0.0.1:8080/history/sync";
      payload = StringFormat("{\"firstTime\":%I64d,\"lastTime\":%I64d,\"symbol\":\"%s\",\"marketType\":\"%s\",\"resolution\":%d}", firstTimeMs, lastTimeMs, InpSourceSymbol, InpHistoryMarketType, InpHistoryResolution);
      Print("DNSE bridge: Triggering incremental history sync...");
   }

   char data[];
   ArrayResize(data, 0);
   int copied = StringToCharArray(payload, data, 0, WHOLE_ARRAY, CP_UTF8);
   if(copied > 0 && ArraySize(data) > 0)
      ArrayResize(data, copied - 1);

   char result[];
   ArrayResize(result, 0);
   string result_headers;
   string headers = "Content-Type: application/json\r\n";

   ResetLastError();
   int res = WebRequest("POST", url, headers, InpHistoryTimeoutMs, data, result, result_headers);
   string response = SafeCharArrayToString(result);
   if(res == 200)
   {
      candlesSynced = ParseCandlesSynced(response);
      Print("DNSE bridge: History sync completed. Response: ", response);
      return true;
   }

   PrintFormat("DNSE bridge: History sync failed. res=%d, GetLastError()=%d, body=%s", res, GetLastError(), response);
   if(res == -1)
   {
      Print("DNSE bridge: Please add 'http://127.0.0.1:8080' to the allowed WebRequest URLs in MT5 (Tools -> Options -> Expert Advisors).");
   }
   return false;
}

void MaybeBackfillRecentHistory()
{
   if(!g_first_tick_seen || g_recent_history_done || InpAutoRecentHistoryDays <= 0)
      return;
   if(TimeCurrent() < g_recent_history_not_before || TimeCurrent() < g_recent_history_retry_after)
      return;

   int candlesSynced = -1;
   if(RequestHistory(true, 0, 0, InpAutoRecentHistoryDays, candlesSynced))
   {
      if(candlesSynced > 0)
      {
         RefreshBridgeHistory();
         int imported = ImportHistoricalRates();
         if(imported > 0)
         {
            g_recent_history_done = true;
            PrintFormat("DNSE bridge: Recent history backfill imported %d candle(s).", imported);
         }
         else
         {
            g_recent_history_retry_after = TimeCurrent() + 60;
            Print("DNSE bridge: Recent history request succeeded but nothing was imported; will retry later.");
         }
      }
      else
      {
         g_recent_history_done = true;
         Print("DNSE bridge: Recent history already up to date.");
      }
   }
   else
   {
      g_recent_history_retry_after = TimeCurrent() + 60;
   }
}

void UpdateRealtimeStatus(bool connected, long timestamp_ms, double last)
{
   string status = connected ? "CONNECTED" : "WAITING";
   string tickText = "no tick yet";
   if(timestamp_ms > 0)
   {
      datetime tickTime = (datetime)(timestamp_ms / 1000);
      long ageSec = (long)(TimeCurrent() - tickTime);
      if(ageSec < 0)
         ageSec = 0;
      tickText = StringFormat("last=%.1f, tick=%s, age=%d sec", last, TimeToString(tickTime, TIME_DATE|TIME_SECONDS), ageSec);
   }
   Comment(StringFormat("DNSE Bridge %s\n%s", status, tickText));
}

void RefreshBridgeHistory()
{
   Print("DNSE bridge: Refreshing DLL bridge to import staged history.");
   DisconnectBridge();
   Sleep(300);
   g_last_reconnect = 0;
   TryConnect();
}

int ImportHistoricalRates()
{
   const int chunkSize = 2000;
   const int waitMs = 60000;
   int waited = 0;

   while(waited < waitMs)
   {
      if(IsHistoryReady())
         break;
      Sleep(200);
      waited += 200;
   }

   if(!IsHistoryReady())
   {
      Print("DNSE bridge: history stream was not marked ready before timeout, importing whatever is available.");
   }

   int total = GetHistoricalRatesCount();
   if(total <= 0)
   {
      Print("DNSE bridge: no historical candles received from DLL.");
      return 0;
   }

   int imported = 0;
   for(int offset = 0; offset < total; offset += chunkSize)
   {
      int want = MathMin(chunkSize, total - offset);
      MqlRateLite historyBuffer[];
      ArrayResize(historyBuffer, want);
      int count = GetHistoricalRatesRange(offset, historyBuffer, want);
      if(count <= 0)
         break;

      MqlRates rates[];
      ArrayResize(rates, count);
      for(int i = 0; i < count; i++)
      {
         rates[i].time = (datetime)(historyBuffer[i].time / 1000);
         rates[i].open = historyBuffer[i].open;
         rates[i].high = historyBuffer[i].high;
         rates[i].low = historyBuffer[i].low;
         rates[i].close = historyBuffer[i].close;
         rates[i].tick_volume = historyBuffer[i].tick_volume;
         rates[i].spread = 0;
         rates[i].real_volume = 0;
      }

      int updated = CustomRatesUpdate(InpCustomSymbol, rates, count);
      if(updated < 0)
      {
         int err = GetLastError();
         PrintFormat("DNSE bridge: CustomRatesUpdate failed at offset=%d, error=%d", offset, err);
         ResetLastError();
         break;
      }
      imported += count;
   }

   ClearHistoricalRates();
   PrintFormat("DNSE bridge: Imported %d/%d historical candles into %s.", imported, total, InpCustomSymbol);
   return imported;
}

bool TryConnect()
{
   if(IsConnected())
   {
      if(g_warned_disconnect)
      {
         Print("DNSE bridge: Socket connected!");
         g_warned_disconnect = false;
      }
      return true;
   }

   datetime now = TimeCurrent();
   if(g_last_reconnect != 0 && (now - g_last_reconnect) < InpReconnectSec)
      return false;

   g_last_reconnect = now;
   PrintFormat("DNSE bridge: Calling ConnectBridge(\"%s\")", InpEndpoint);
   bool ok = ConnectBridge(InpEndpoint);
   if(ok)
   {
      PrintFormat("DNSE bridge: ConnectBridge started thread for %s", InpEndpoint);
   }
   else
   {
      PrintFormat("DNSE bridge: ConnectBridge failed for %s", InpEndpoint);
   }
   return ok;
}

int OnInit()
{
   if(!MQLInfoInteger(MQL_DLLS_ALLOWED))
   {
      Print("DNSE bridge: DLL imports are disabled. Enable 'Allow DLL imports' in EA settings.");
      return INIT_FAILED;
   }
   if(!EnsureCustomSymbol())
      return INIT_FAILED;

   TryConnect();
   g_first_tick_seen = false;
   g_recent_history_done = (InpAutoRecentHistoryDays <= 0);
   g_recent_history_not_before = TimeCurrent() + InpAutoRecentSyncDelaySec;
   g_recent_history_retry_after = 0;
   EventSetMillisecondTimer(MathMax(50, InpTimerMs));
   PrintFormat("DNSE bridge v1.11: EA started, source=%s, custom symbol=%s, autoRecentHistoryDays=%d, historyTimeoutMs=%d", InpSourceSymbol, InpCustomSymbol, InpAutoRecentHistoryDays, InpHistoryTimeoutMs);
   if(InpAutoRecentHistoryDays <= 0)
      Print("DNSE bridge v1.11: auto history backfill is disabled; realtime is priority, older history is manual.");
   return INIT_SUCCEEDED;
}

void OnDeinit(const int reason)
{
   EventKillTimer();
   DisconnectBridge();
   Print("DNSE bridge: EA stopped");
}

void OnTimer()
{
   PollPendingSignals();

   if(!TryConnect())
   {
      if(!g_warned_disconnect)
      {
         Print("DNSE bridge: waiting for DLL/socket connection");
         g_warned_disconnect = true;
      }
      UpdateRealtimeStatus(false, g_last_timestamp_ms, g_last_last);
      return;
   }

   if(IsHistoryReady() && GetHistoricalRatesCount() > 0)
   {
      ImportHistoricalRates();
   }

   double bid = 0.0;
   double ask = 0.0;
   double last = 0.0;
   long volume = 0;
   long timestamp_ms = 0;

   if(!GetLatestTick(bid, ask, last, volume, timestamp_ms))
   {
      UpdateRealtimeStatus(true, g_last_timestamp_ms, g_last_last);
      return;
   }
   if(timestamp_ms <= 0)
   {
      UpdateRealtimeStatus(true, g_last_timestamp_ms, g_last_last);
      return;
   }

   if(timestamp_ms == g_last_timestamp_ms && bid == g_last_bid && ask == g_last_ask && last == g_last_last && volume == g_last_volume)
   {
      MaybeBackfillRecentHistory();
      UpdateRealtimeStatus(true, timestamp_ms, last);
      return;
   }

   MqlTick tick;
   ZeroMemory(tick);
   tick.time      = (datetime)(timestamp_ms / 1000);
   tick.time_msc  = timestamp_ms;
   tick.bid       = bid;
   tick.ask       = ask;
   tick.last      = last;
   tick.volume    = volume;
   tick.volume_real = (double)volume;
   tick.flags     = TICK_FLAG_BID | TICK_FLAG_ASK | TICK_FLAG_LAST | TICK_FLAG_VOLUME;

   MqlTick ticks[1];
   ticks[0] = tick;
   int added = CustomTicksAdd(InpCustomSymbol, ticks);
   if(added > 0)
   {
      g_last_timestamp_ms = timestamp_ms;
      g_last_bid = bid;
      g_last_ask = ask;
      g_last_last = last;
      g_last_volume = volume;
      g_first_tick_seen = true;
      UpdateRealtimeStatus(true, timestamp_ms, last);
      if(g_last_realtime_log == 0 || (TimeCurrent() - g_last_realtime_log) >= 30)
      {
         PrintFormat("DNSE bridge: realtime tick applied. last=%.1f, bid=%.1f, ask=%.1f, time=%s", last, bid, ask, TimeToString((datetime)(timestamp_ms / 1000), TIME_DATE|TIME_SECONDS));
         g_last_realtime_log = TimeCurrent();
      }
      
      CheckForSignal();
      MaybeBackfillRecentHistory();
   }
   else
   {
      int err = GetLastError();
      PrintFormat("DNSE bridge: CustomTicksAdd failed, error=%d", err);
      ResetLastError();
   }
}
