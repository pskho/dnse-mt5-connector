//+------------------------------------------------------------------+
//| DNSE MT5 Market Data Bridge                                      |
//| Updates custom symbol VN30F1M_DNSE from DNSEBridge.dll           |
//+------------------------------------------------------------------+
#property strict
#property version   "1.14"

struct MqlRateLite {
   long     time;
   double   open;
   double   high;
   double   low;
   double   close;
   long     tick_volume;
};

#import "DNSEBridge.dll"
bool ConnectBridge(string endpoint, string symbol);
bool IsConnected(string symbol);
bool IsHistoryReady(string symbol);
bool GetLatestTick(string symbol, double &bid, double &ask, double &last, long &volume, long &timestamp_ms);
void DisconnectBridge(string symbol);
int GetHistoricalRates(string symbol, MqlRateLite &buffer[], int max_count);
int GetHistoricalRatesCount(string symbol);
int GetHistoricalRatesRange(string symbol, int offset, MqlRateLite &buffer[], int max_count);
void ClearHistoricalRates(string symbol);
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
input bool   InpEnableManualTradePanel = true;
input string InpManualBridgeURL = "http://127.0.0.1:8080";
input string InpManualAccountNos = "ENTRADE_DEMO"; // comma-separated account/profile ids
input int    InpManualQuantity = 1;
input string InpManualOrderType = "MTL"; // MTL or LO
input double InpManualLimitPrice = 0.0;   // for LO; 0 uses latest realtime price
input string InpManualMarketType = "DERIVATIVE";
input string InpManualOrderCategory = "NORMAL";

string   g_instance_lock_name = "DNSE_MT5_BRIDGE_MASTER_LOCK";
string   g_manual_panel_prefix = "DNSE_MANUAL_TRADE_";
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

string   g_symbols[];
string   g_custom_symbols[];
string   g_layout_symbols[];
string   g_layout_groups[];
string   g_layout_descriptions[];
int      g_layout_digits[];
double   g_layout_points[];
long     g_last_timestamp_by_symbol[];
double   g_last_bid_by_symbol[];
double   g_last_ask_by_symbol[];
double   g_last_last_by_symbol[];
long     g_last_volume_by_symbol[];
bool     g_history_imported_by_symbol[];
datetime g_last_realtime_log_by_symbol[];
datetime g_last_reconnect_by_symbol[];

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
   string payload = StringFormat("{\"action\":\"%s\",\"symbol\":\"%s\",\"side\":\"%s\",\"quantity\":1,\"source\":\"MT5\"}", side, InpSourceSymbol, side);
   
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

void SendCloseDealSignal()
{
   string url = "http://127.0.0.1:8080/signal";
   string payload = StringFormat("{\"action\":\"CLOSE_DEAL\",\"symbol\":\"%s\",\"orderType\":\"MTL\",\"source\":\"MT5\"}", InpSourceSymbol);
   
   char data[];
   StringToCharArray(payload, data, 0, StringLen(payload), CP_UTF8);
   
   char result[];
   string result_headers;
   string headers = "Content-Type: application/json\r\n";
   
   int res = WebRequest("POST", url, headers, 1000, data, result, result_headers);
   string response = CharArrayToString(result, 0, WHOLE_ARRAY, CP_UTF8);
   if(res == 200)
      PrintFormat("DNSE bridge: Close-deal signal sent. Response: %s", response);
   else
      PrintFormat("DNSE bridge: Failed to send close-deal signal. Code: %d, Msg: %s", res, response);
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

void CreateManualTradeButton(string name, string text, int x, int y, int w, int h, color bg, color fg)
{
   if(ObjectFind(0, name) < 0)
      ObjectCreate(0, name, OBJ_BUTTON, 0, 0, 0);
   ObjectSetInteger(0, name, OBJPROP_CORNER, CORNER_RIGHT_UPPER);
   ObjectSetInteger(0, name, OBJPROP_XDISTANCE, x);
   ObjectSetInteger(0, name, OBJPROP_YDISTANCE, y);
   ObjectSetInteger(0, name, OBJPROP_XSIZE, w);
   ObjectSetInteger(0, name, OBJPROP_YSIZE, h);
   ObjectSetInteger(0, name, OBJPROP_BGCOLOR, bg);
   ObjectSetInteger(0, name, OBJPROP_COLOR, fg);
   ObjectSetInteger(0, name, OBJPROP_BORDER_COLOR, clrWhite);
   ObjectSetInteger(0, name, OBJPROP_FONTSIZE, 10);
   ObjectSetString(0, name, OBJPROP_FONT, "Arial Bold");
   ObjectSetString(0, name, OBJPROP_TEXT, text);
   ObjectSetInteger(0, name, OBJPROP_SELECTABLE, false);
   ObjectSetInteger(0, name, OBJPROP_HIDDEN, true);
}

void CreateManualTradeLabel(string name, string text, int x, int y, color fg)
{
   if(ObjectFind(0, name) < 0)
      ObjectCreate(0, name, OBJ_LABEL, 0, 0, 0);
   ObjectSetInteger(0, name, OBJPROP_CORNER, CORNER_RIGHT_UPPER);
   ObjectSetInteger(0, name, OBJPROP_XDISTANCE, x);
   ObjectSetInteger(0, name, OBJPROP_YDISTANCE, y);
   ObjectSetInteger(0, name, OBJPROP_COLOR, fg);
   ObjectSetInteger(0, name, OBJPROP_FONTSIZE, 8);
   ObjectSetString(0, name, OBJPROP_FONT, "Arial");
   ObjectSetString(0, name, OBJPROP_TEXT, text);
   ObjectSetInteger(0, name, OBJPROP_SELECTABLE, false);
   ObjectSetInteger(0, name, OBJPROP_HIDDEN, true);
}

void DrawManualTradePanel()
{
   if(!InpEnableManualTradePanel)
      return;
   CreateManualTradeButton(g_manual_panel_prefix + "BUY", "BUY", 172, 28, 72, 28, clrGreen, clrWhite);
   CreateManualTradeButton(g_manual_panel_prefix + "SELL", "SELL", 94, 28, 72, 28, clrRed, clrWhite);
   string label = StringFormat("%s x%d", InpManualOrderType, InpManualQuantity);
   CreateManualTradeLabel(g_manual_panel_prefix + "INFO", label, 94, 60, clrSilver);
   ChartRedraw(0);
}

void DeleteManualTradePanel()
{
   int total = ObjectsTotal(0, 0, -1);
   for(int i = total - 1; i >= 0; i--)
   {
      string name = ObjectName(0, i, 0, -1);
      if(StringFind(name, g_manual_panel_prefix) == 0)
         ObjectDelete(0, name);
   }
}

string JSONEscape(string text)
{
   StringReplace(text, "\\", "\\\\");
   StringReplace(text, "\"", "\\\"");
   return text;
}

string BuildManualAccountsJSON()
{
   string raw = InpManualAccountNos;
   StringTrimLeft(raw);
   StringTrimRight(raw);
   if(raw == "")
      raw = "ENTRADE_DEMO";

   string parts[];
   int count = StringSplit(raw, ',', parts);
   if(count <= 1)
      return "\"accountNo\":\"" + JSONEscape(raw) + "\"";

   string out = "\"accountNos\":[";
   int added = 0;
   for(int i = 0; i < count; i++)
   {
      string value = parts[i];
      StringTrimLeft(value);
      StringTrimRight(value);
      if(value == "")
         continue;
      if(added > 0)
         out += ",";
      out += "\"" + JSONEscape(value) + "\"";
      added++;
   }
   out += "]";
   if(added == 0)
      return "\"accountNo\":\"ENTRADE_DEMO\"";
   return out;
}

bool SendManualOrder(string side)
{
   if(!InpEnableManualTradePanel)
      return false;
   side = NormalizeSymbolName(side);
   if(side != "BUY" && side != "SELL")
      return false;

   string orderType = InpManualOrderType;
   StringTrimLeft(orderType);
   StringTrimRight(orderType);
   StringToUpper(orderType);
   if(orderType == "")
      orderType = "MTL";

   double price = InpManualLimitPrice;
   if(orderType == "LO" && price <= 0.0)
      price = g_last_last;

   string clientOrderId = StringFormat("mt5-manual-%s-%I64d", side, GetTickCount64());
   string payload = "{";
   payload += "\"clientOrderId\":\"" + JSONEscape(clientOrderId) + "\"";
   payload += "," + BuildManualAccountsJSON();
   payload += ",\"symbol\":\"" + JSONEscape(InpSourceSymbol) + "\"";
   payload += ",\"side\":\"" + side + "\"";
   payload += ",\"quantity\":" + IntegerToString((int)MathMax(1, InpManualQuantity));
   payload += ",\"price\":" + DoubleToString(MathMax(0.0, price), LookupDigits(InpSourceSymbol));
   payload += ",\"orderType\":\"" + JSONEscape(orderType) + "\"";
   payload += ",\"marketType\":\"" + JSONEscape(InpManualMarketType) + "\"";
   payload += ",\"orderCategory\":\"" + JSONEscape(InpManualOrderCategory) + "\"";
   payload += "}";

   string baseURL = InpManualBridgeURL;
   StringTrimRight(baseURL);
   while(StringLen(baseURL) > 0 && StringSubstr(baseURL, StringLen(baseURL) - 1, 1) == "/")
      baseURL = StringSubstr(baseURL, 0, StringLen(baseURL) - 1);
   string url = baseURL + "/order";
   char data[];
   ArrayResize(data, 0);
   int copied = StringToCharArray(payload, data, 0, WHOLE_ARRAY, CP_UTF8);
   if(copied > 0 && ArraySize(data) > 0)
      ArrayResize(data, copied - 1);

   char result[];
   string result_headers;
   string headers = "Content-Type: application/json\r\n";
   ResetLastError();
   int res = WebRequest("POST", url, headers, 3000, data, result, result_headers);
   string response = SafeCharArrayToString(result);
   if(res == 200)
   {
      PrintFormat("DNSE manual order %s submitted. clientOrderId=%s, response=%s", side, clientOrderId, response);
      return true;
   }
   PrintFormat("DNSE manual order %s failed. HTTP=%d, error=%d, body=%s", side, res, GetLastError(), response);
   if(res == -1)
      Print("DNSE bridge: add http://127.0.0.1:8080 to MT5 WebRequest allowed URLs.");
   return false;
}

bool EnsureCustomSymbol()
{
   string groupPath = LookupGroupPath(InpSourceSymbol);
   if(groupPath == "")
      groupPath = "DNSE";
   if(!SymbolSelect(InpCustomSymbol, true) && !CustomSymbolCreate(InpCustomSymbol, groupPath))
   {
      int err = GetLastError();
      PrintFormat("DNSE bridge: cannot create custom symbol %s, error=%d", InpCustomSymbol, err);
      ResetLastError();
      return false;
   }

   CustomSymbolSetInteger(InpCustomSymbol, SYMBOL_DIGITS, LookupDigits(InpSourceSymbol));
   CustomSymbolSetDouble(InpCustomSymbol, SYMBOL_POINT, LookupPoint(InpSourceSymbol));
   CustomSymbolSetString(InpCustomSymbol, SYMBOL_DESCRIPTION, LookupDescription(InpSourceSymbol));
   SymbolSelect(InpCustomSymbol, true);
   return true;
}

void EnsureNamedCustomSymbol(string sourceSymbol)
{
   sourceSymbol = NormalizeSymbolName(sourceSymbol);
   string customSymbol = sourceSymbol + "_DNSE";
   string groupPath = LookupGroupPath(sourceSymbol);
   if(groupPath == "")
      groupPath = "DNSE";
   if(!SymbolSelect(customSymbol, true) && !CustomSymbolCreate(customSymbol, groupPath))
      return;
   CustomSymbolSetInteger(customSymbol, SYMBOL_DIGITS, LookupDigits(sourceSymbol));
   CustomSymbolSetDouble(customSymbol, SYMBOL_POINT, LookupPoint(sourceSymbol));
   CustomSymbolSetString(customSymbol, SYMBOL_DESCRIPTION, LookupDescription(sourceSymbol));
   SymbolSelect(customSymbol, true);
}

string NormalizeSymbolName(string value)
{
   StringTrimLeft(value);
   StringTrimRight(value);
   StringToUpper(value);
   if(StringFind(value, "VN100F") == 0)
      value = "V100F" + StringSubstr(value, 6);
   return value;
}

int FindTrackedSymbolIndex(string symbol)
{
   symbol = NormalizeSymbolName(symbol);
   for(int i = 0; i < ArraySize(g_symbols); i++)
   {
      if(g_symbols[i] == symbol)
         return i;
   }
   return -1;
}

void AddTrackedSymbol(string sourceSymbol)
{
   sourceSymbol = NormalizeSymbolName(sourceSymbol);
   if(sourceSymbol == "")
      return;
   if(FindTrackedSymbolIndex(sourceSymbol) >= 0)
      return;

   int idx = ArraySize(g_symbols);
   ArrayResize(g_symbols, idx + 1);
   ArrayResize(g_custom_symbols, idx + 1);
   ArrayResize(g_last_timestamp_by_symbol, idx + 1);
   ArrayResize(g_last_bid_by_symbol, idx + 1);
   ArrayResize(g_last_ask_by_symbol, idx + 1);
   ArrayResize(g_last_last_by_symbol, idx + 1);
   ArrayResize(g_last_volume_by_symbol, idx + 1);
   ArrayResize(g_history_imported_by_symbol, idx + 1);
   ArrayResize(g_last_realtime_log_by_symbol, idx + 1);
   ArrayResize(g_last_reconnect_by_symbol, idx + 1);

   g_symbols[idx] = sourceSymbol;
   g_custom_symbols[idx] = sourceSymbol + "_DNSE";
   g_last_timestamp_by_symbol[idx] = 0;
   g_last_bid_by_symbol[idx] = 0.0;
   g_last_ask_by_symbol[idx] = 0.0;
   g_last_last_by_symbol[idx] = 0.0;
   g_last_volume_by_symbol[idx] = 0;
   g_history_imported_by_symbol[idx] = false;
   g_last_realtime_log_by_symbol[idx] = 0;
   g_last_reconnect_by_symbol[idx] = 0;
}

int FindLayoutIndex(string symbol)
{
   symbol = NormalizeSymbolName(symbol);
   for(int i = 0; i < ArraySize(g_layout_symbols); i++)
   {
      if(g_layout_symbols[i] == symbol)
         return i;
   }
   return -1;
}

string LookupGroupPath(string symbol)
{
   int idx = FindLayoutIndex(symbol);
   if(idx >= 0 && idx < ArraySize(g_layout_groups) && g_layout_groups[idx] != "")
      return g_layout_groups[idx];
   return "DNSE";
}

string LookupDescription(string symbol)
{
   symbol = NormalizeSymbolName(symbol);
   int idx = FindLayoutIndex(symbol);
   if(idx >= 0 && idx < ArraySize(g_layout_descriptions) && g_layout_descriptions[idx] != "")
      return g_layout_descriptions[idx];
   return "DNSE realtime " + symbol;
}

int LookupDigits(string symbol)
{
   int idx = FindLayoutIndex(symbol);
   if(idx >= 0 && idx < ArraySize(g_layout_digits) && g_layout_digits[idx] > 0)
      return g_layout_digits[idx];
   symbol = NormalizeSymbolName(symbol);
   if(StringFind(symbol, "VN30F") == 0 || StringFind(symbol, "V100F") == 0)
      return 1;
   return 2;
}

double LookupPoint(string symbol)
{
   int idx = FindLayoutIndex(symbol);
   if(idx >= 0 && idx < ArraySize(g_layout_points) && g_layout_points[idx] > 0.0)
      return g_layout_points[idx];
   symbol = NormalizeSymbolName(symbol);
   if(StringFind(symbol, "VN30F") == 0 || StringFind(symbol, "V100F") == 0)
      return 0.1;
   return 0.01;
}

void LoadMT5Layouts()
{
   ArrayResize(g_layout_symbols, 0);
   ArrayResize(g_layout_groups, 0);
   ArrayResize(g_layout_descriptions, 0);
   ArrayResize(g_layout_digits, 0);
   ArrayResize(g_layout_points, 0);

   string url = "http://127.0.0.1:8080/symbols/mt5-layout";
   char data[];
   char result[];
   string result_headers;
   int res = WebRequest("GET", url, "", 3000, data, result, result_headers);
   if(res != 200)
   {
      PrintFormat("DNSE bridge: cannot load MT5 symbol layout. res=%d", res);
      return;
   }

   string response = CharArrayToString(result, 0, WHOLE_ARRAY, CP_UTF8);
   string rows[];
   int rowCount = StringSplit(response, '\n', rows);
   for(int i = 0; i < rowCount; i++)
   {
      string row = rows[i];
      StringTrimLeft(row);
      StringTrimRight(row);
      if(row == "")
         continue;

      string cols[];
      int colCount = StringSplit(row, '\t', cols);
      if(colCount < 3)
         continue;

      string symbol = NormalizeSymbolName(cols[0]);
      if(symbol == "")
         continue;

      int idx = ArraySize(g_layout_symbols);
      ArrayResize(g_layout_symbols, idx + 1);
      ArrayResize(g_layout_groups, idx + 1);
      ArrayResize(g_layout_descriptions, idx + 1);
      ArrayResize(g_layout_digits, idx + 1);
      ArrayResize(g_layout_points, idx + 1);
      g_layout_symbols[idx] = symbol;
      g_layout_groups[idx] = cols[1];
      g_layout_descriptions[idx] = cols[2];
      g_layout_digits[idx] = (colCount >= 4) ? (int)StringToInteger(cols[3]) : LookupDigits(symbol);
      g_layout_points[idx] = (colCount >= 5) ? StringToDouble(cols[4]) : LookupPoint(symbol);
   }
}

string GetConfiguredSymbolsSummary()
{
   string summary = "";
   for(int i = 0; i < ArraySize(g_symbols); i++)
   {
      if(i > 0)
         summary += ", ";
      summary += g_symbols[i];
   }
   return summary;
}

void EnsureConfiguredCustomSymbols()
{
   LoadMT5Layouts();
   ArrayResize(g_symbols, 0);
   ArrayResize(g_custom_symbols, 0);
   ArrayResize(g_last_timestamp_by_symbol, 0);
   ArrayResize(g_last_bid_by_symbol, 0);
   ArrayResize(g_last_ask_by_symbol, 0);
   ArrayResize(g_last_last_by_symbol, 0);
   ArrayResize(g_last_volume_by_symbol, 0);
   ArrayResize(g_history_imported_by_symbol, 0);
   ArrayResize(g_last_realtime_log_by_symbol, 0);
   ArrayResize(g_last_reconnect_by_symbol, 0);

   AddTrackedSymbol(InpSourceSymbol);
   string url = "http://127.0.0.1:8080/symbols/profiles";
   char data[];
   char result[];
   string result_headers;
   int res = WebRequest("GET", url, "", 3000, data, result, result_headers);
   if(res != 200)
      return;

   string response = CharArrayToString(result, 0, WHOLE_ARRAY, CP_UTF8);
   int pos = 0;
   while(true)
   {
      int start = StringFind(response, "\"Symbol\":\"", pos);
      if(start < 0)
         break;
      start += 10;
      int end = StringFind(response, "\"", start);
      if(end <= start)
         break;
      string sourceSymbol = StringSubstr(response, start, end - start);
      if(sourceSymbol != "")
      {
         AddTrackedSymbol(sourceSymbol);
         EnsureNamedCustomSymbol(sourceSymbol);
      }
      pos = end + 1;
   }
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
   DisconnectBridge(InpSourceSymbol);
   Sleep(300);
   g_last_reconnect = 0;
   TryConnect();
}

int ImportHistoricalRates()
{
   long latestImportedTimeMs = 0;
   return ImportHistoricalRatesForSymbol(InpSourceSymbol, InpCustomSymbol, latestImportedTimeMs);
}

int ImportHistoricalRatesForSymbol(string sourceSymbol, string customSymbol, long &latestImportedTimeMs)
{
   const int chunkSize = 2000;
   const int waitMs = 60000;
   int waited = 0;
   latestImportedTimeMs = 0;

   while(waited < waitMs)
   {
      if(IsHistoryReady(sourceSymbol))
         break;
      Sleep(200);
      waited += 200;
   }

   if(!IsHistoryReady(sourceSymbol))
   {
      PrintFormat("DNSE bridge: history stream for %s was not marked ready before timeout, importing whatever is available.", sourceSymbol);
   }

   int total = GetHistoricalRatesCount(sourceSymbol);
   if(total <= 0)
   {
      PrintFormat("DNSE bridge: no historical candles received from DLL for %s.", sourceSymbol);
      return 0;
   }

   int imported = 0;
   for(int offset = 0; offset < total; offset += chunkSize)
   {
      int want = MathMin(chunkSize, total - offset);
      MqlRateLite historyBuffer[];
      ArrayResize(historyBuffer, want);
      int count = GetHistoricalRatesRange(sourceSymbol, offset, historyBuffer, want);
      if(count <= 0)
         break;

      MqlRates rates[];
      ArrayResize(rates, count);
      for(int i = 0; i < count; i++)
      {
         rates[i].time = (datetime)(historyBuffer[i].time / 1000);
         if(historyBuffer[i].time > latestImportedTimeMs)
            latestImportedTimeMs = historyBuffer[i].time;
         rates[i].open = historyBuffer[i].open;
         rates[i].high = historyBuffer[i].high;
         rates[i].low = historyBuffer[i].low;
         rates[i].close = historyBuffer[i].close;
         rates[i].tick_volume = historyBuffer[i].tick_volume;
         rates[i].spread = 0;
         rates[i].real_volume = 0;
      }

      if(count > 0)
      {
         datetime deleteFrom = rates[0].time;
         datetime deleteTo = rates[count - 1].time;
         if(deleteTo >= deleteFrom)
            CustomRatesDelete(customSymbol, deleteFrom, deleteTo);
      }

      int updated = CustomRatesUpdate(customSymbol, rates, count);
      if(updated < 0)
      {
         int err = GetLastError();
         PrintFormat("DNSE bridge: CustomRatesUpdate failed for %s at offset=%d, error=%d", customSymbol, offset, err);
         ResetLastError();
         break;
      }
      imported += count;
   }

   ClearHistoricalRates(sourceSymbol);
   PrintFormat("DNSE bridge: Imported %d/%d historical candles into %s.", imported, total, customSymbol);
   return imported;
}

bool TryConnect()
{
   return TryConnectSymbol(InpSourceSymbol, true);
}

bool TryConnectSymbol(string sourceSymbol, bool logPrimary)
{
   int idx = FindTrackedSymbolIndex(sourceSymbol);
   if(idx < 0)
      return false;

   if(IsConnected(sourceSymbol))
   {
      if(logPrimary && g_warned_disconnect)
      {
         Print("DNSE bridge: Socket connected!");
         g_warned_disconnect = false;
      }
      return true;
   }

   datetime now = TimeCurrent();
   if(g_last_reconnect_by_symbol[idx] != 0 && (now - g_last_reconnect_by_symbol[idx]) < InpReconnectSec)
      return false;

   g_last_reconnect_by_symbol[idx] = now;
   if(logPrimary)
      PrintFormat("DNSE bridge: Calling ConnectBridge(\"%s\", \"%s\")", InpEndpoint, sourceSymbol);
   bool ok = ConnectBridge(InpEndpoint, sourceSymbol);
   if(logPrimary)
   {
      if(ok)
         PrintFormat("DNSE bridge: ConnectBridge started thread for %s", InpEndpoint);
      else
         PrintFormat("DNSE bridge: ConnectBridge failed for %s", InpEndpoint);
   }
   return ok;
}

bool IsDerivativeRealtimeSymbol(string symbol)
{
   symbol = NormalizeSymbolName(symbol);
   return StringFind(symbol, "VN30F") == 0 ||
          StringFind(symbol, "V100F") == 0 ||
          StringFind(symbol, "VNF") == 0 ||
          StringFind(symbol, "F1M") >= 0 ||
          StringFind(symbol, "F2M") >= 0 ||
          StringFind(symbol, "F1Q") >= 0 ||
          StringFind(symbol, "F2Q") >= 0;
}

bool IsHOSEIndexRealtimeSymbol(string symbol)
{
   symbol = NormalizeSymbolName(symbol);
   return symbol == "VNINDEX" ||
          symbol == "VN30" ||
          symbol == "VN100" ||
          symbol == "VNXALLSHARE" ||
          symbol == "VNDIVIDEND" ||
          symbol == "VN50GROWTH" ||
          symbol == "VNMITECH";
}

bool IsHNXUPCOMRealtimeSymbol(string symbol)
{
   symbol = NormalizeSymbolName(symbol);
   return symbol == "HNX" || symbol == "HNX30" || symbol == "UPCOM";
}

bool NormalizeKRXRealtimeTimestamp(string sourceSymbol, long &timestamp_ms)
{
   if(timestamp_ms <= 0)
      return false;

   int morningOpen = 8 * 60 + 45;
   int openingAuctionEnd = 0;
   int morningClose = 11 * 60 + 30;
   int afternoonOpen = 13 * 60;
   int closingAuction = 0;
   int afternoonClose = 15 * 60;

   if(IsDerivativeRealtimeSymbol(sourceSymbol))
   {
      morningOpen = 8 * 60 + 45;
      openingAuctionEnd = 9 * 60;
      closingAuction = 14 * 60 + 30;
      afternoonClose = 14 * 60 + 45;
   }
   else if(IsHOSEIndexRealtimeSymbol(sourceSymbol))
   {
      morningOpen = 9 * 60;
      openingAuctionEnd = 9 * 60 + 15;
      closingAuction = 14 * 60 + 30;
      afternoonClose = 14 * 60 + 45;
   }
   else if(IsHNXUPCOMRealtimeSymbol(sourceSymbol))
   {
      morningOpen = 9 * 60;
      afternoonClose = 15 * 60;
   }

   datetime vnTime = (datetime)(timestamp_ms / 1000 + 7 * 3600);
   MqlDateTime dt;
   TimeToStruct(vnTime, dt);
   if(dt.day_of_week == 0 || dt.day_of_week == 6)
      return false;

   int minute = dt.hour * 60 + dt.min;
   if(minute < morningOpen || minute > afternoonClose)
      return false;
   if(minute > morningClose && minute < afternoonOpen)
      return false;

   int bucketMinute = minute;
   if(openingAuctionEnd > 0 && minute >= morningOpen && minute < openingAuctionEnd)
      bucketMinute = morningOpen;
   if(closingAuction > 0 && minute >= closingAuction && minute <= afternoonClose)
      bucketMinute = closingAuction;

   if(bucketMinute != minute)
   {
      dt.hour = bucketMinute / 60;
      dt.min = bucketMinute % 60;
      dt.sec = 0;
      timestamp_ms = (long)(StructToTime(dt) - 7 * 3600) * 1000;
   }
   return true;
}

void ProcessTrackedSymbol(int idx)
{
   string sourceSymbol = g_symbols[idx];
   string customSymbol = g_custom_symbols[idx];

   double bid = 0.0;
   double ask = 0.0;
   double last = 0.0;
   long volume = 0;
   long timestamp_ms = 0;
   if(!GetLatestTick(sourceSymbol, bid, ask, last, volume, timestamp_ms))
      return;
   if(timestamp_ms <= 0)
      return;
   if(!NormalizeKRXRealtimeTimestamp(sourceSymbol, timestamp_ms))
      return;

   if(timestamp_ms == g_last_timestamp_by_symbol[idx] &&
      bid == g_last_bid_by_symbol[idx] &&
      ask == g_last_ask_by_symbol[idx] &&
      last == g_last_last_by_symbol[idx] &&
      volume == g_last_volume_by_symbol[idx])
      return;

   if(timestamp_ms <= g_last_timestamp_by_symbol[idx])
      timestamp_ms = g_last_timestamp_by_symbol[idx] + 1;

   MqlTick tick;
   ZeroMemory(tick);
   tick.time = (datetime)(timestamp_ms / 1000);
   tick.time_msc = timestamp_ms;
   tick.bid = bid;
   tick.ask = ask;
   tick.last = last;
   tick.volume = volume;
   tick.volume_real = (double)volume;
   tick.flags = TICK_FLAG_BID | TICK_FLAG_ASK | TICK_FLAG_LAST | TICK_FLAG_VOLUME;

   MqlTick ticks[1];
   ticks[0] = tick;
   int added = CustomTicksAdd(customSymbol, ticks);
   if(added <= 0)
   {
      int err = GetLastError();
      if(err == 5310)
      {
         PrintFormat("DNSE bridge: CustomTicksAdd skipped for %s because MT5 rejected the incoming tick order/time (error=%d).", customSymbol, err);
         ResetLastError();
         g_last_timestamp_by_symbol[idx] = timestamp_ms;
         g_last_bid_by_symbol[idx] = bid;
         g_last_ask_by_symbol[idx] = ask;
         g_last_last_by_symbol[idx] = last;
         g_last_volume_by_symbol[idx] = volume;
         return;
      }
      PrintFormat("DNSE bridge: CustomTicksAdd failed for %s, error=%d", customSymbol, err);
      ResetLastError();
      return;
   }

   g_last_timestamp_by_symbol[idx] = timestamp_ms;
   g_last_bid_by_symbol[idx] = bid;
   g_last_ask_by_symbol[idx] = ask;
   g_last_last_by_symbol[idx] = last;
   g_last_volume_by_symbol[idx] = volume;
   if(sourceSymbol == InpSourceSymbol)
      g_first_tick_seen = true;

   if(g_last_realtime_log_by_symbol[idx] == 0 || (TimeCurrent() - g_last_realtime_log_by_symbol[idx]) >= 30)
   {
      PrintFormat("DNSE bridge: realtime tick applied for %s. last=%.1f, bid=%.1f, ask=%.1f, time=%s", sourceSymbol, last, bid, ask, TimeToString((datetime)(timestamp_ms / 1000), TIME_DATE|TIME_SECONDS));
      g_last_realtime_log_by_symbol[idx] = TimeCurrent();
   }

   if(sourceSymbol == InpSourceSymbol)
   {
      g_last_timestamp_ms = timestamp_ms;
      g_last_bid = bid;
      g_last_ask = ask;
      g_last_last = last;
      g_last_volume = volume;
      UpdateRealtimeStatus(true, timestamp_ms, last);
      CheckForSignal();
   }

   if(IsHistoryReady(sourceSymbol) && GetHistoricalRatesCount(sourceSymbol) > 0)
   {
      long latestImportedTimeMs = 0;
      int imported = ImportHistoricalRatesForSymbol(sourceSymbol, customSymbol, latestImportedTimeMs);
      if(imported > 0)
      {
         g_history_imported_by_symbol[idx] = true;
         if(latestImportedTimeMs > g_last_timestamp_by_symbol[idx])
            g_last_timestamp_by_symbol[idx] = latestImportedTimeMs;
      }
   }
}

int OnInit()
{
   if(!MQLInfoInteger(MQL_DLLS_ALLOWED))
   {
      Print("DNSE bridge: DLL imports are disabled. Enable 'Allow DLL imports' in EA settings.");
      return INIT_FAILED;
   }
   if(GlobalVariableCheck(g_instance_lock_name))
   {
      Print("DNSE bridge: Another EA instance is already running. Keep only one master EA attached to one chart. If this is a stale lock, press F3 in MT5 and delete DNSE_MT5_BRIDGE_MASTER_LOCK.");
      return INIT_FAILED;
   }
   GlobalVariableSet(g_instance_lock_name, (double)ChartID());
   if(!EnsureCustomSymbol())
   {
      Print("DNSE bridge: OnInit failed because the primary custom symbol could not be created or selected.");
      GlobalVariableDel(g_instance_lock_name);
      return INIT_FAILED;
   }
   EnsureConfiguredCustomSymbols();
   for(int i = 0; i < ArraySize(g_symbols); i++)
      TryConnectSymbol(g_symbols[i], g_symbols[i] == InpSourceSymbol);
   g_first_tick_seen = false;
   g_recent_history_done = (InpAutoRecentHistoryDays <= 0);
   g_recent_history_not_before = TimeCurrent() + InpAutoRecentSyncDelaySec;
   g_recent_history_retry_after = 0;
   DrawManualTradePanel();
   EventSetMillisecondTimer(MathMax(50, InpTimerMs));
   PrintFormat("DNSE bridge v1.14: EA started, source=%s, custom symbol=%s, trackedSymbols=%s, autoRecentHistoryDays=%d, historyTimeoutMs=%d, manualTradePanel=%s, manualAccounts=%s", InpSourceSymbol, InpCustomSymbol, GetConfiguredSymbolsSummary(), InpAutoRecentHistoryDays, InpHistoryTimeoutMs, InpEnableManualTradePanel ? "ON" : "OFF", InpManualAccountNos);
   if(InpAutoRecentHistoryDays <= 0)
      Print("DNSE bridge v1.14: auto history backfill is disabled; realtime is priority, older history is manual.");
   return INIT_SUCCEEDED;
}

void OnDeinit(const int reason)
{
   EventKillTimer();
   DeleteManualTradePanel();
   for(int i = 0; i < ArraySize(g_symbols); i++)
      DisconnectBridge(g_symbols[i]);
   GlobalVariableDel(g_instance_lock_name);
   Print("DNSE bridge: EA stopped");
}

void OnTimer()
{
   PollPendingSignals();

   bool primaryConnected = TryConnectSymbol(InpSourceSymbol, true);
   for(int i = 0; i < ArraySize(g_symbols); i++)
   {
      if(g_symbols[i] == InpSourceSymbol)
         continue;
      TryConnectSymbol(g_symbols[i], false);
   }

   if(!primaryConnected)
   {
      if(!g_warned_disconnect)
      {
         Print("DNSE bridge: waiting for DLL/socket connection");
         g_warned_disconnect = true;
      }
      UpdateRealtimeStatus(false, g_last_timestamp_ms, g_last_last);
      return;
   }

   for(int i = 0; i < ArraySize(g_symbols); i++)
      ProcessTrackedSymbol(i);

   MaybeBackfillRecentHistory();
   UpdateRealtimeStatus(true, g_last_timestamp_ms, g_last_last);
}

void OnChartEvent(const int id,
                  const long &lparam,
                  const double &dparam,
                  const string &sparam)
{
   if(id != CHARTEVENT_OBJECT_CLICK)
      return;
   if(sparam == g_manual_panel_prefix + "BUY")
   {
      ObjectSetInteger(0, sparam, OBJPROP_STATE, false);
      SendManualOrder("BUY");
      return;
   }
   if(sparam == g_manual_panel_prefix + "SELL")
   {
      ObjectSetInteger(0, sparam, OBJPROP_STATE, false);
      SendManualOrder("SELL");
      return;
   }
}
