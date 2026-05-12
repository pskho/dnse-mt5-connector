//+------------------------------------------------------------------+
//| DNSE SuperTrend Bot Example                                      |
//| Sends BUY/SELL signals to the local DNSE/Entrade bridge.         |
//+------------------------------------------------------------------+
#property strict
#property version "1.01"

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
input bool            InpShowChartObjects   = true;
input int             InpVisualBars         = 200;
input int             InpTrendLineWidth     = 2;
input color           InpUpTrendColor       = clrLime;
input color           InpDownTrendColor     = clrTomato;
input color           InpBuyMarkerColor     = clrLime;
input color           InpSellMarkerColor    = clrTomato;
input bool            InpKeepDrawingsOnStop = false;

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
   if(!InpKeepDrawingsOnStop)
      ClearChartObjects();
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

   DrawSuperTrendObjects(rates, supertrend, direction, copied);

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

void DrawSuperTrendObjects(MqlRates &rates[], double &supertrend[], int &direction[], int count)
{
   if(!InpShowChartObjects)
      return;
   if(count < InpATRPeriod + 5)
      return;

   int drawBars = InpVisualBars;
   if(drawBars > count - 2)
      drawBars = count - 2;
   if(drawBars < 2)
      return;

   string prefix = ChartObjectPrefix();
   DeleteObjectsByPrefix(prefix + "LINE_");
   DeleteObjectsByPrefix(prefix + "SIGNAL_");

   for(int i = drawBars; i >= 2; i--)
   {
      if(supertrend[i] <= 0.0 || supertrend[i - 1] <= 0.0)
         continue;
      color lineColor = direction[i - 1] >= 0 ? InpUpTrendColor : InpDownTrendColor;
      DrawTrendSegment(prefix, rates[i].time, supertrend[i], rates[i - 1].time, supertrend[i - 1], lineColor);
   }

   for(int i = drawBars - 1; i >= 1; i--)
   {
      bool crossUp = rates[i + 1].close <= supertrend[i + 1] &&
                     rates[i].close > supertrend[i] &&
                     direction[i] == 1 &&
                     direction[i + 1] == -1;
      bool crossDown = rates[i + 1].close >= supertrend[i + 1] &&
                       rates[i].close < supertrend[i] &&
                       direction[i] == -1 &&
                       direction[i + 1] == 1;

      if(crossUp)
         DrawSignalMarker(prefix, "BUY", rates[i].time, rates[i].low, rates[i].close, InpBuyMarkerColor, true);
      else if(crossDown)
         DrawSignalMarker(prefix, "SELL", rates[i].time, rates[i].high, rates[i].close, InpSellMarkerColor, false);
   }

   ChartRedraw(0);
}

void DrawTrendSegment(string prefix, datetime time1, double price1, datetime time2, double price2, color lineColor)
{
   string name = prefix + "LINE_" + IntegerToString((long)time1) + "_" + IntegerToString((long)time2);
   if(!ObjectCreate(0, name, OBJ_TREND, 0, time1, price1, time2, price2))
      return;
   int width = InpTrendLineWidth;
   if(width < 1)
      width = 1;
   ObjectSetInteger(0, name, OBJPROP_COLOR, lineColor);
   ObjectSetInteger(0, name, OBJPROP_WIDTH, width);
   ObjectSetInteger(0, name, OBJPROP_STYLE, STYLE_SOLID);
   ObjectSetInteger(0, name, OBJPROP_RAY_LEFT, false);
   ObjectSetInteger(0, name, OBJPROP_RAY_RIGHT, false);
   ObjectSetInteger(0, name, OBJPROP_SELECTABLE, false);
   ObjectSetInteger(0, name, OBJPROP_HIDDEN, true);
}

void DrawSignalMarker(string prefix, string side, datetime signalTime, double anchorPrice, double tradePrice, color markerColor, bool isBuy)
{
   double point = SymbolInfoDouble(InpCustomSymbol, SYMBOL_POINT);
   if(point <= 0.0)
      point = 0.1;
   double markerOffset = point * 12.0;
   double textOffset = point * 24.0;
   double markerPrice = isBuy ? anchorPrice - markerOffset : anchorPrice + markerOffset;
   double textPrice = isBuy ? anchorPrice - textOffset : anchorPrice + textOffset;

   string signalKey = IntegerToString((long)signalTime) + "_" + side;
   string arrowName = prefix + "SIGNAL_ARROW_" + signalKey;
   string textName = prefix + "SIGNAL_TEXT_" + signalKey;

   if(ObjectCreate(0, arrowName, OBJ_ARROW, 0, signalTime, markerPrice))
   {
      ObjectSetInteger(0, arrowName, OBJPROP_ARROWCODE, isBuy ? 233 : 234);
      ObjectSetInteger(0, arrowName, OBJPROP_COLOR, markerColor);
      ObjectSetInteger(0, arrowName, OBJPROP_WIDTH, 2);
      ObjectSetInteger(0, arrowName, OBJPROP_SELECTABLE, false);
      ObjectSetInteger(0, arrowName, OBJPROP_HIDDEN, true);
   }

   int digits = (int)SymbolInfoInteger(InpCustomSymbol, SYMBOL_DIGITS);
   if(digits <= 0)
      digits = 1;
   string label = side + " " + DoubleToString(tradePrice, digits);
   if(ObjectCreate(0, textName, OBJ_TEXT, 0, signalTime, textPrice))
   {
      ObjectSetString(0, textName, OBJPROP_TEXT, label);
      ObjectSetString(0, textName, OBJPROP_FONT, "Arial");
      ObjectSetInteger(0, textName, OBJPROP_FONTSIZE, 9);
      ObjectSetInteger(0, textName, OBJPROP_COLOR, markerColor);
      ObjectSetInteger(0, textName, OBJPROP_SELECTABLE, false);
      ObjectSetInteger(0, textName, OBJPROP_HIDDEN, true);
   }
}

void ClearChartObjects()
{
   string prefix = ChartObjectPrefix();
   DeleteObjectsByPrefix(prefix + "LINE_");
   DeleteObjectsByPrefix(prefix + "SIGNAL_");
}

void DeleteObjectsByPrefix(string prefix)
{
   int total = ObjectsTotal(0, 0, -1);
   for(int i = total - 1; i >= 0; i--)
   {
      string name = ObjectName(0, i, 0, -1);
      if(StringFind(name, prefix) == 0)
         ObjectDelete(0, name);
   }
}

string ChartObjectPrefix()
{
   return "DNSE_ST_" + InpCustomSymbol + "_" + EnumToString(InpTimeframe) + "_";
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
