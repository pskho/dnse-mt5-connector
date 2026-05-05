package marketdata

import (
	"strconv"
	"strings"

	"dnse-mt5-connector/internal/config"
)

type SymbolProfile struct {
	Symbol               string
	AssetClass           string
	MarketType           string
	Channels             []string
	Resolution           int
	BoardID              string
	SupportsRESTFallback bool
}

func DefaultTrackedSymbols() []string {
	return []string{
		"VN30F1M",
		"VN30F2M",
		"VN30F1Q",
		"VN30F2Q",
		"V100F1M",
		"V100F2M",
		"V100F1Q",
		"V100F2Q",
		"VNINDEX",
		"VN30",
		"HNX",
		"HNX30",
		"VN100",
		"UPCOM",
		"VNXALLSHARE",
	}
}

func BuildProfiles(cfg config.MarketDataConfig) []SymbolProfile {
	symbols := cfg.Symbols
	if len(symbols) == 0 && strings.TrimSpace(cfg.Symbol) != "" {
		symbols = []string{cfg.Symbol}
	}
	out := make([]SymbolProfile, 0, len(symbols))
	for _, symbol := range symbols {
		if profile, ok := InferSymbolProfile(symbol, cfg.Channels); ok {
			out = append(out, profile)
		}
	}
	return out
}

func InferSymbolProfile(symbol string, configuredChannels []string) (SymbolProfile, bool) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if strings.HasPrefix(symbol, "VN100F") {
		symbol = "V100F" + strings.TrimPrefix(symbol, "VN100F")
	}
	if symbol == "" {
		return SymbolProfile{}, false
	}

	profile := SymbolProfile{
		Symbol:     symbol,
		Resolution: 1,
	}
	switch {
	case isIndexSymbol(symbol):
		profile.AssetClass = "INDEX"
		profile.MarketType = "INDEX"
		profile.Channels = []string{"ohlc.1.json", "market_index." + symbol + ".json"}
		profile.SupportsRESTFallback = false
	case isDerivativeSymbol(symbol):
		profile.AssetClass = "DERIVATIVE"
		profile.MarketType = "DERIVATIVE"
		profile.BoardID = "G1"
		profile.Channels = []string{"tick.G1.json", "top_price.G1.json", "ohlc.1.json", "ohlc_closed.1.json"}
		profile.SupportsRESTFallback = true
	default:
		profile.AssetClass = "STOCK"
		profile.MarketType = "STOCK"
		profile.BoardID = "G1"
		profile.Channels = []string{"tick.G1.json", "top_price.G1.json", "ohlc.1.json", "ohlc_closed.1.json"}
		profile.SupportsRESTFallback = true
	}
	if len(configuredChannels) > 0 && !isIndexSymbol(symbol) {
		profile.Channels = normalizeChannelList(configuredChannels, profile.BoardID, profile.Resolution)
	}
	return profile, true
}

func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func normalizeChannelList(values []string, boardID string, resolution int) []string {
	if boardID == "" {
		boardID = "G1"
	}
	if resolution <= 0 {
		resolution = 1
	}
	out := make([]string, 0, len(values)*2)
	seen := make(map[string]struct{})
	add := func(channel string) {
		channel = strings.TrimSpace(channel)
		if channel == "" {
			return
		}
		if _, ok := seen[channel]; ok {
			return
		}
		seen[channel] = struct{}{}
		out = append(out, channel)
	}
	for _, value := range values {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "trades.json", "trade.json", "tick.json", "trades":
			add("tick." + boardID + ".json")
		case "trade_extra.json", "trades_extra.json", "tick_extra.json":
			add("tick_extra." + boardID + ".json")
		case "quotes.json", "quote.json", "top_price.json", "quotes":
			add("top_price." + boardID + ".json")
		case "ohlc.json", "ohlc":
			add("ohlc." + strconv.Itoa(resolution) + ".json")
		case "ohlc_closed.json", "ohlc_closed":
			add("ohlc_closed." + strconv.Itoa(resolution) + ".json")
		default:
			add(strings.TrimSpace(value))
		}
	}
	if len(out) == 0 {
		add("tick." + boardID + ".json")
		add("top_price." + boardID + ".json")
	}
	return out
}

func isDerivativeSymbol(symbol string) bool {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	return strings.HasPrefix(symbol, "VN30F") ||
		strings.HasPrefix(symbol, "V100F") ||
		strings.HasPrefix(symbol, "VNF") ||
		looksLikeDerivativeFeedCode(symbol) ||
		strings.Contains(symbol, "F1M") ||
		strings.Contains(symbol, "F2M") ||
		strings.Contains(symbol, "F1Q") ||
		strings.Contains(symbol, "F2Q")
}

func isIndexSymbol(symbol string) bool {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	switch symbol {
	case "VNINDEX", "VN30", "HNX", "HNX30", "UPCOM", "VNXALLSHARE", "VN100", "VNDIVIDEND", "VN50GROWTH", "VNMITECH":
		return true
	default:
		return false
	}
}

func looksLikeDerivativeFeedCode(symbol string) bool {
	if len(symbol) < 6 {
		return false
	}
	if !(symbol[0] >= '0' && symbol[0] <= '9' && symbol[1] >= '0' && symbol[1] <= '9') {
		return false
	}
	if symbol[2] != 'I' || !(symbol[3] >= '0' && symbol[3] <= '9') {
		return false
	}
	return strings.Contains(symbol, "G") || strings.Contains(symbol, "C")
}
