//+------------------------------------------------------------------+
//| DNSE SuperTrend Bot Example                                      |
//| Sends BUY/SELL signals to the local DNSE/Entrade bridge.         |
//+------------------------------------------------------------------+
#property strict
#property version "1.00"

input string          InpBridgeURL          = "http://127.0.0.1:8080";
input string          InpSourceSymbol       = "VN30F1M";
input string          InpCustomSymbol       = "VN30F1M_DNSE";
input ENUM_TIMEFRAMES InpTimeframe          = PERIOD_M1;
input int             InpATRPeriod         = 10;
input double          InpMultiplier        = 3.0;
input int             InpQuantity          = 1;
input string          InpOrderType         = "MTL"; // MTL, LO, MAK, MOK, ATO, ATC
input double          InpLimitPrice        = 0.0;   // used only when InpOrderType=LO
input bool            InpCloseBeforeReverse = true;
input bool            InpEnableTrading      = false;
input int             InpTimerSeconds       = 1;
input int             InpBarsToCalculate    = 300;

int      g_atr_handle = INVALID_HANDLE;
datetime g_last_closed_bar_time = 0;
int      g_last_direction = 0;

int OnInit()
{
   if(!SymbolSelect(InpCustomSymbol, true))
   {
      PrintFormat("SuperTrend bot: cannot select custom symbol %s, error=%d", InpCustomSymbol, GetLastError());
      return INIT_FAILED;
   }

   g_atr_handle = iATR(InpCustomSymbol, InpTimeframe, InpATRPeriod);
   if(g_atr_handle == INVALID_HANDLE)
   {
      PrintFormat("SuperTrend bot: cannot create ATR handle, error=%d", GetLastError());
      return INIT_FAILED;
   }

   EventSetTimer((int)MathMax(1, InpTimerSeconds));
   PrintFormat("SuperTrend bot v1.00 started: symbol=%s, timeframe=%s, atr=%d, multiplier=%.2f, trading=%s",
               InpCustomSymbol,
               EnumToString(InpTimeframe),
               InpATRPeriod,
               InpMultiplier,
               InpEnableTrading ? "ON" : "OFF");
   return INIT_SUCCEEDED;
}

void OnDeinit(const int reason)
{
   EventKillTimer();
   if(g_atr_handle != INVALID_HANDLE)
      IndicatorRelease(g_atr_handle);
   Print("SuperTrend bot stopped.");
}

void OnTimer()
{
   CheckSuperTrendSignal();
}

void CheckSuperTrendSignal()
{
   int barsNeeded = (int)MathMax(InpBarsToCalculate, InpATRPeriod + 50);
   MqlRates rates[];
   ArraySetAsSeries(rates, true);
   int copied = CopyRates(InpCustomSymbol, InpTimeframe, 0, barsNeeded, rates);
   if(copied < InpATRPeriod + 5)
      return;

   double atr[];
   ArraySetAsSeries(atr, true);
   int atrCopied = CopyBuffer(g_atr_handle, 0, 0, copied, atr);
   if(atrCopied < copied - 1)
      return;

   datetime closedBarTime = rates[1].time;
   if(closedBarTime <= 0 || closedBarTime == g_last_closed_bar_time)
      return;

   double supertrend[];
   int direction[];
   ArrayResize(supertrend, copied);
   ArrayResize(direction, copied);

   if(!CalculateSuperTrend(rates, atr, copied, supertrend, direction))
      return;

   int currentDirection = direction[1];
   int previousDirection = direction[2];
   double currentClose = rates[1].close;
   double previousClose = rates[2].close;
   double currentST = supertrend[1];
   double previousST = supertrend[2];

   g_last_closed_bar_time = closedBarTime;

   if(g_last_direction == 0)
   {
      g_last_direction = currentDirection;
      PrintFormat("SuperTrend bot initialized direction=%s close=%.2f st=%.2f",
                  DirectionName(currentDirection), currentClose, currentST);
      return;
   }

   bool crossUp = previousClose <= previousST && currentClose > currentST && currentDirection == 1 && previousDirection == -1;
   bool crossDown = previousClose >= previousST && currentClose < currentST && currentDirection == -1 && previousDirection == 1;

   if(crossUp)
   {
      PrintFormat("SuperTrend BUY signal: close %.2f crossed above ST %.2f at %s",
                  currentClose, currentST, TimeToString(closedBarTime, TIME_DATE|TIME_SECONDS));
      if(InpEnableTrading)
      {
         if(InpCloseBeforeReverse)
            SendCloseDealSignal();
         SendOrderSignal("BUY");
      }
   }
   else if(crossDown)
   {
      PrintFormat("SuperTrend SELL signal: close %.2f crossed below ST %.2f at %s",
                  currentClose, currentST, TimeToString(closedBarTime, TIME_DATE|TIME_SECONDS));
      if(InpEnableTrading)
      {
         if(InpCloseBeforeReverse)
            SendCloseDealSignal();
         SendOrderSignal("SELL");
      }
   }

   g_last_direction = currentDirection;
   Comment(StringFormat("SuperTrend %s\nclose=%.2f st=%.2f\nlast bar=%s\ntrading=%s",
                        DirectionName(currentDirection),
                        currentClose,
                        currentST,
                        TimeToString(closedBarTime, TIME_DATE|TIME_SECONDS),
                        InpEnableTrading ? "ON" : "OFF"));
}

bool CalculateSuperTrend(MqlRates &rates[], double &atr[], int count, double &supertrend[], int &direction[])
{
   if(count < InpATRPeriod + 5)
      return false;

   double finalUpper[];
   double finalLower[];
   ArrayResize(finalUpper, count);
   ArrayResize(finalLower, count);

   int oldest = count - 1;
   double hl2 = (rates[oldest].high + rates[oldest].low) / 2.0;
   finalUpper[oldest] = hl2 + InpMultiplier * atr[oldest];
   finalLower[oldest] = hl2 - InpMultiplier * atr[oldest];
   direction[oldest] = rates[oldest].close >= hl2 ? 1 : -1;
   supertrend[oldest] = direction[oldest] == 1 ? finalLower[oldest] : finalUpper[oldest];

   for(int i = oldest - 1; i >= 1; i--)
   {
      if(atr[i] <= 0.0)
         return false;

      double basicUpper = ((rates[i].high + rates[i].low) / 2.0) + InpMultiplier * atr[i];
      double basicLower = ((rates[i].high + rates[i].low) / 2.0) - InpMultiplier * atr[i];

      if(basicUpper < finalUpper[i + 1] || rates[i + 1].close > finalUpper[i + 1])
         finalUpper[i] = basicUpper;
      else
         finalUpper[i] = finalUpper[i + 1];

      if(basicLower > finalLower[i + 1] || rates[i + 1].close < finalLower[i + 1])
         finalLower[i] = basicLower;
      else
         finalLower[i] = finalLower[i + 1];

      if(direction[i + 1] == -1)
      {
         if(rates[i].close > finalUpper[i])
            direction[i] = 1;
         else
            direction[i] = -1;
      }
      else
      {
         if(rates[i].close < finalLower[i])
            direction[i] = -1;
         else
            direction[i] = 1;
      }

      supertrend[i] = direction[i] == 1 ? finalLower[i] : finalUpper[i];
   }

   return true;
}

void SendOrderSignal(string side)
{
   string action = side;
   string payload = "{";
   payload += "\"action\":\"" + action + "\"";
   payload += ",\"symbol\":\"" + InpSourceSymbol + "\"";
   payload += ",\"side\":\"" + side + "\"";
   payload += ",\"quantity\":" + IntegerToString(InpQuantity);
   string orderType = InpOrderType;
   StringToUpper(orderType);
   payload += ",\"orderType\":\"" + orderType + "\"";
   if(orderType == "LO" && InpLimitPrice > 0.0)
      payload += ",\"price\":" + DoubleToString(InpLimitPrice, 1);
   payload += "}";

   SendJSON("/signal", payload, "order " + side);
}

void SendCloseDealSignal()
{
   string payload = "{";
   payload += "\"action\":\"CLOSE_DEAL\"";
   payload += ",\"symbol\":\"" + InpSourceSymbol + "\"";
   payload += ",\"orderType\":\"" + InpOrderType + "\"";
   payload += "}";

   SendJSON("/signal", payload, "close deal");
}

bool SendJSON(string path, string payload, string label)
{
   string url = InpBridgeURL + path;
   char data[];
   StringToCharArray(payload, data, 0, StringLen(payload), CP_UTF8);

   char result[];
   string resultHeaders;
   string headers = "Content-Type: application/json\r\n";
   ResetLastError();
   int res = WebRequest("POST", url, headers, 3000, data, result, resultHeaders);
   string response = CharArrayToString(result, 0, WHOLE_ARRAY, CP_UTF8);

   if(res == 200)
   {
      PrintFormat("SuperTrend bot: %s signal sent. Response: %s", label, response);
      return true;
   }

   PrintFormat("SuperTrend bot: failed to send %s signal. HTTP=%d, error=%d, body=%s",
               label, res, GetLastError(), response);
   if(res == -1)
      Print("SuperTrend bot: add http://127.0.0.1:8080 to MT5 WebRequest allowed URLs.");
   return false;
}

string DirectionName(int direction)
{
   if(direction > 0)
      return "UP";
   if(direction < 0)
      return "DOWN";
   return "UNKNOWN";
}
