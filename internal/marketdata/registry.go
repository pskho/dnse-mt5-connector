package marketdata

import (
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
		profile.Channels = []string{"ohlc.1.json"}
		profile.SupportsRESTFallback = false
	case isDerivativeSymbol(symbol):
		profile.AssetClass = "DERIVATIVE"
		profile.MarketType = "DERIVATIVE"
		profile.BoardID = "G1"
		profile.Channels = []string{"trades.json", "quotes.json"}
		profile.SupportsRESTFallback = true
	default:
		profile.AssetClass = "STOCK"
		profile.MarketType = "STOCK"
		profile.BoardID = "G1"
		profile.Channels = []string{"trades.json", "quotes.json"}
		profile.SupportsRESTFallback = true
	}
	if len(configuredChannels) > 0 && !isIndexSymbol(symbol) {
		profile.Channels = normalizeList(configuredChannels)
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

func isDerivativeSymbol(symbol string) bool {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	return strings.HasPrefix(symbol, "VN30F") || strings.HasPrefix(symbol, "VNF") || strings.Contains(symbol, "F1M") || strings.Contains(symbol, "F2M")
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
