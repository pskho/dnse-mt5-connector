package marketdata

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

func NormalizeTick(symbol string, raw []byte) (Tick, bool) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return Tick{}, false
	}
	return normalizeAny(strings.ToUpper(symbol), value)
}

func normalizeAny(expectedSymbol string, value any) (Tick, bool) {
	switch v := value.(type) {
	case map[string]any:
		for _, key := range []string{"data", "payload", "tick", "quote", "trade", "d", "content", "result"} {
			if nested, ok := v[key]; ok {
				if tick, ok := normalizeAny(expectedSymbol, nested); ok {
					return tick, true
				}
			}
		}
		for _, key := range []string{"items", "ticks", "quotes", "trades", "list", "rows"} {
			if arr, ok := v[key].([]any); ok {
				for _, item := range arr {
					if tick, ok := normalizeAny(expectedSymbol, item); ok {
						return tick, true
					}
				}
			}
		}
		return normalizeMap(expectedSymbol, v)
	case []any:
		for _, item := range v {
			if tick, ok := normalizeAny(expectedSymbol, item); ok {
				return tick, true
			}
		}
	}
	return Tick{}, false
}

func normalizeMap(expectedSymbol string, item map[string]any) (Tick, bool) {
	if isBarCloseEvent(item) {
		return Tick{}, false
	}

	source := messageSource(item)
	symbol := strings.ToUpper(firstString(item, "symbol", "indexName", "s", "secSymbol", "code", "ticker", "sym"))
	if symbol == "" {
		symbol = expectedSymbol
	}
	if expectedSymbol != "" && symbol != expectedSymbol {
		return Tick{}, false
	}

	last := firstFloat(item, "last", "lastPrice", "price", "matchPrice", "close", "valueIndexes", "c", "lp", "mp", "p", "lastMatchedPrice")
	bid := firstFloat(item, "bid", "bestBid", "bidPrice", "bp", "b", "bid1", "b1", "bestBid1", "bp1")
	ask := firstFloat(item, "ask", "bestAsk", "askPrice", "ap", "a", "ask1", "a1", "bestAsk1", "ap1", "offer1")
	if bid <= 0 {
		bid = firstLevelPrice(item, "bid")
	}
	if ask <= 0 {
		ask = firstLevelPrice(item, "offer", "ask")
	}
	if last <= 0 && bid <= 0 && ask <= 0 {
		return Tick{}, false
	}
	if source == "quote" && last <= 0 {
		// Quote is depth-of-book data. Do not invent a last trade from the
		// midpoint; Store.Update will merge bid/ask with the previous trade.
		last = 0
	} else {
		if bid <= 0 {
			bid = last
		}
		if ask <= 0 {
			ask = last
		}
		if last <= 0 && bid > 0 && ask > 0 {
			last = (bid + ask) / 2
		}
	}

	ts := firstInt64(item, "timestamp_ms", "timestampMs", "time", "ts", "t", "tradingTime", "matchTime", "tradeTime", "updateTime", "tt")
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}
	if ts > 0 && ts < 100000000000 {
		ts *= 1000
	}

	return Tick{
		Symbol:      symbol,
		Bid:         bid,
		Ask:         ask,
		Last:        last,
		Volume:      firstInt64(item, "volume", "vol", "matchVolume", "lastQuantity", "quantity", "q", "tradeQuantity", "tq", "lastVolume", "totalVolumeTraded", "contauctAccTrdVol"),
		TimestampMS: ts,
		Source:      source,
	}, true
}

func messageSource(item map[string]any) string {
	eventType := strings.ToLower(firstString(item, "T", "event", "eventType", "type"))
	switch eventType {
	case "t", "trade", "tick":
		return "trade"
	case "te", "trade_extra", "tick_extra":
		return "trade_extra"
	case "q", "quote", "top_price":
		return "quote"
	case "b", "bar", "ohlc":
		return "ohlc"
	case "mi", "market_index":
		return "market_index"
	}
	if _, ok := item["bid"]; ok {
		return "quote"
	}
	if _, ok := item["offer"]; ok {
		return "quote"
	}
	if _, ok := item["price"]; ok {
		return "trade"
	}
	if _, ok := item["matchPrice"]; ok {
		return "trade"
	}
	if _, ok := item["resolution"]; ok {
		return "ohlc"
	}
	return "unknown"
}

func firstLevelPrice(item map[string]any, keys ...string) float64 {
	for _, key := range keys {
		raw, ok := item[key]
		if !ok {
			continue
		}
		arr, ok := raw.([]any)
		if !ok || len(arr) == 0 {
			continue
		}
		level, ok := arr[0].(map[string]any)
		if !ok {
			continue
		}
		price := firstFloat(level, "price", "p")
		if price > 0 {
			return price
		}
	}
	return 0
}

func isBarCloseEvent(item map[string]any) bool {
	eventType := strings.ToLower(firstString(item, "T", "event", "eventType"))
	if eventType == "b" || eventType == "bar" || eventType == "ohlc" {
		return false
	}
	if eventType == "bc" || eventType == "bar_close" || eventType == "ohlc_closed" {
		return true
	}
	if _, hasResolution := item["resolution"]; hasResolution {
		if _, hasOpen := item["open"]; hasOpen {
			if _, hasClose := item["close"]; hasClose {
				return eventType == ""
			}
		}
	}
	return false
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch v := value.(type) {
			case string:
				return strings.TrimSpace(v)
			case float64:
				return strconv.FormatInt(int64(v), 10)
			}
		}
	}
	return ""
}

func firstFloat(item map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch v := value.(type) {
			case float64:
				return v
			case int:
				return float64(v)
			case int64:
				return float64(v)
			case string:
				n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
				return n
			}
		}
	}
	return 0
}

func firstInt64(item map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch v := value.(type) {
			case float64:
				return int64(v)
			case int:
				return int64(v)
			case int64:
				return v
			case string:
				n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
				return n
			}
		}
	}
	return 0
}
