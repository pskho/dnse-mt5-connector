package config

import "testing"

func TestNormalizeTradingConfigKeepsCustomGroupsWithoutReaddingEntradeDemo(t *testing.T) {
	in := TradingConfig{
		Groups: []ExecutionGroupConfig{
			{
				ID:              "custom-real",
				Name:            "Custom Real",
				Enabled:         true,
				AccountNos:      []string{"ENTRADE_REAL_1000000279"},
				DefaultQuantity: 1,
				MaxQuantity:     1,
				OrderType:       "MTL",
				MarketType:      "DERIVATIVE",
				OrderCategory:   "NORMAL",
				AllowBuy:        true,
				AllowSell:       true,
			},
		},
		Routes: TradingRoutesConfig{
			Dashboard:  "custom-real",
			MT5Manual:  "custom-real",
			SuperTrend: "custom-real",
			SignalAPI:  "custom-real",
			OrderAPI:   "custom-real",
		},
	}

	got := normalizeTradingConfig(in, "", []string{"ENTRADE_DEMO"})

	if len(got.Groups) != 1 {
		t.Fatalf("expected exactly one custom group, got %d: %#v", len(got.Groups), got.Groups)
	}
	if got.Groups[0].ID != "custom-real" {
		t.Fatalf("expected custom group to be preserved, got %q", got.Groups[0].ID)
	}
	if got.Routes.MT5Manual != "custom-real" {
		t.Fatalf("expected MT5 route to stay custom-real, got %q", got.Routes.MT5Manual)
	}
}
