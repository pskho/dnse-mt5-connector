package marketdata

import (
	"testing"
	"time"

	"dnse-mt5-connector/internal/storage"
)

func TestCacheRecordsCompleteForRangeDetectsMinuteGap(t *testing.T) {
	from := time.Date(2026, 5, 14, 9, 15, 0, 0, vnLocation).UnixMilli()
	to := time.Date(2026, 5, 14, 9, 18, 30, 0, vnLocation).UnixMilli()
	records := []storage.HistoryCandleRecord{
		{TimeMS: from},
		{TimeMS: from + 2*60_000},
	}

	if cacheRecordsCompleteForRange(records, 1, from, to, hoseKRXPolicy()) {
		t.Fatal("expected cache completeness check to reject a missing 1m candle")
	}
}

func TestCacheRecordsCompleteForRangeAcceptsContinuousSession(t *testing.T) {
	from := time.Date(2026, 5, 14, 9, 15, 0, 0, vnLocation).UnixMilli()
	to := time.Date(2026, 5, 14, 9, 18, 30, 0, vnLocation).UnixMilli()
	records := []storage.HistoryCandleRecord{
		{TimeMS: from},
		{TimeMS: from + 60_000},
		{TimeMS: from + 2*60_000},
	}

	if !cacheRecordsCompleteForRange(records, 1, from, to, hoseKRXPolicy()) {
		t.Fatal("expected continuous 1m cache to be usable")
	}
}

func TestFillTradingSessionGapsForwardFillsWithinSameSession(t *testing.T) {
	first := time.Date(2026, 5, 14, 9, 15, 0, 0, vnLocation).UnixMilli()
	candles := []HistoryCandle{
		{Time: first, Open: 100, High: 100, Low: 100, Close: 100},
		{Time: first + 2*60_000, Open: 102, High: 102, Low: 102, Close: 102},
	}

	filled := fillTradingSessionGapsWithPolicy(candles, 1, hoseKRXPolicy())
	if len(filled) != 3 {
		t.Fatalf("expected 3 candles after filling one gap, got %d", len(filled))
	}
	if filled[1].Time != first+60_000 || filled[1].Close != 100 {
		t.Fatalf("unexpected filled candle: %+v", filled[1])
	}
}

func TestFillTradingSessionGapsForRangeFillsAcrossLunchBreak(t *testing.T) {
	morning := time.Date(2026, 5, 14, 9, 29, 0, 0, vnLocation).UnixMilli()
	afternoon := time.Date(2026, 5, 14, 13, 1, 0, 0, vnLocation).UnixMilli()
	candles := []HistoryCandle{
		{Time: morning, Open: 100, High: 100, Low: 100, Close: 100},
		{Time: afternoon, Open: 101, High: 101, Low: 101, Close: 101},
	}

	filled := fillTradingSessionGapsForRange(candles, 1, morning, afternoon, hoseKRXPolicy())
	seen := map[int64]HistoryCandle{}
	for _, candle := range filled {
		seen[candle.Time] = candle
	}

	morningGap := time.Date(2026, 5, 14, 9, 30, 0, 0, vnLocation).UnixMilli()
	afternoonOpen := time.Date(2026, 5, 14, 13, 0, 0, 0, vnLocation).UnixMilli()
	lunch := time.Date(2026, 5, 14, 12, 0, 0, 0, vnLocation).UnixMilli()
	if _, ok := seen[morningGap]; !ok {
		t.Fatal("expected missing morning minute to be filled before lunch")
	}
	if _, ok := seen[afternoonOpen]; !ok {
		t.Fatal("expected afternoon session start to be filled")
	}
	if _, ok := seen[lunch]; ok {
		t.Fatal("did not expect non-trading lunch minute to be filled")
	}
}

func TestATCWindowIsSingleCandleForDerivativeAndHOSEPolicy(t *testing.T) {
	atcStart := time.Date(2026, 5, 14, 14, 30, 0, 0, vnLocation).UnixMilli()
	afterATCStart := time.Date(2026, 5, 14, 14, 31, 0, 0, vnLocation).UnixMilli()
	atcEnd := time.Date(2026, 5, 14, 14, 45, 0, 0, vnLocation).UnixMilli()
	policy := hoseKRXPolicy()

	if !isVNTradingTimestampMSWithPolicy(atcStart, policy) {
		t.Fatal("expected 14:30 to be the single ATC candle")
	}
	if isVNTradingTimestampMSWithPolicy(afterATCStart, policy) {
		t.Fatal("did not expect 14:31 to be treated as a missing 1m candle")
	}
	if isVNTradingTimestampMSWithPolicy(atcEnd, policy) {
		t.Fatal("did not expect 14:45 to be treated as a separate 1m candle")
	}
}

func TestNormalizeKRXCandlesCollapsesDerivativeATC(t *testing.T) {
	atcStart := time.Date(2026, 5, 14, 14, 30, 0, 0, vnLocation).UnixMilli()
	atcMiddle := time.Date(2026, 5, 14, 14, 31, 0, 0, vnLocation).UnixMilli()
	atcEnd := time.Date(2026, 5, 14, 14, 45, 0, 0, vnLocation).UnixMilli()
	candles := []HistoryCandle{
		{Time: atcStart, Open: 100, High: 101, Low: 99, Close: 100, TickVolume: 1},
		{Time: atcMiddle, Open: 102, High: 103, Low: 101, Close: 102, TickVolume: 2},
		{Time: atcEnd, Open: 104, High: 105, Low: 103, Close: 104, TickVolume: 3},
	}

	normalized := normalizeKRXCandles(candles, derivativeKRXPolicy())
	if len(normalized) != 1 {
		t.Fatalf("expected ATC candles to collapse into one 14:30 candle, got %d", len(normalized))
	}
	got := normalized[0]
	if got.Time != atcStart || got.Open != 100 || got.High != 105 || got.Low != 99 || got.Close != 104 || got.TickVolume != 6 {
		t.Fatalf("unexpected collapsed ATC candle: %+v", got)
	}
}

func TestNormalizeRealtimeTickTimestampCollapsesDerivativeATC(t *testing.T) {
	atcEnd := time.Date(2026, 5, 14, 14, 45, 0, 0, vnLocation).UnixMilli()
	want := time.Date(2026, 5, 14, 14, 30, 0, 0, vnLocation).UnixMilli()

	got, ok := normalizeRealtimeTickTimestampForSymbolMS("VN30F1M", atcEnd)
	if !ok {
		t.Fatal("expected derivative ATC tick to be accepted")
	}
	if got != want {
		t.Fatalf("expected derivative ATC tick to be bucketed at 14:30, got %s", time.UnixMilli(got).In(vnLocation))
	}
}

func TestHNXAndUPCOMPolicyKeepsTradingUntil1500(t *testing.T) {
	atcStart := time.Date(2026, 5, 14, 14, 30, 0, 0, vnLocation).UnixMilli()
	afterATCStart := time.Date(2026, 5, 14, 14, 31, 0, 0, vnLocation).UnixMilli()
	close1450 := time.Date(2026, 5, 14, 14, 50, 0, 0, vnLocation).UnixMilli()
	close1500 := time.Date(2026, 5, 14, 15, 0, 0, 0, vnLocation).UnixMilli()
	afterClose := time.Date(2026, 5, 14, 15, 1, 0, 0, vnLocation).UnixMilli()
	policy := hnxUPCOMKRXPolicy()

	if !isVNTradingTimestampMSWithPolicy(atcStart, policy) ||
		!isVNTradingTimestampMSWithPolicy(afterATCStart, policy) ||
		!isVNTradingTimestampMSWithPolicy(close1450, policy) ||
		!isVNTradingTimestampMSWithPolicy(close1500, policy) {
		t.Fatal("expected HNX/UPCOM policy to keep minute candles through 15:00")
	}
	if isVNTradingTimestampMSWithPolicy(afterClose, policy) {
		t.Fatal("did not expect HNX/UPCOM policy to treat 15:01 as a trading minute")
	}
}

func TestKRXOpeningAuctionIsSingleCandleForDerivativeAndHOSE(t *testing.T) {
	derivativeOpen := time.Date(2026, 5, 14, 8, 45, 0, 0, vnLocation).UnixMilli()
	derivativeAuctionMiddle := time.Date(2026, 5, 14, 8, 46, 0, 0, vnLocation).UnixMilli()
	derivativeContinuous := time.Date(2026, 5, 14, 9, 0, 0, 0, vnLocation).UnixMilli()
	if !isVNTradingTimestampMSWithPolicy(derivativeOpen, derivativeKRXPolicy()) ||
		isVNTradingTimestampMSWithPolicy(derivativeAuctionMiddle, derivativeKRXPolicy()) ||
		!isVNTradingTimestampMSWithPolicy(derivativeContinuous, derivativeKRXPolicy()) {
		t.Fatal("expected derivative KRX ATO to be represented by a single 08:45 candle")
	}

	hoseOpen := time.Date(2026, 5, 14, 9, 0, 0, 0, vnLocation).UnixMilli()
	hoseAuctionMiddle := time.Date(2026, 5, 14, 9, 1, 0, 0, vnLocation).UnixMilli()
	hoseContinuous := time.Date(2026, 5, 14, 9, 15, 0, 0, vnLocation).UnixMilli()
	if !isVNTradingTimestampMSWithPolicy(hoseOpen, hoseKRXPolicy()) ||
		isVNTradingTimestampMSWithPolicy(hoseAuctionMiddle, hoseKRXPolicy()) ||
		!isVNTradingTimestampMSWithPolicy(hoseContinuous, hoseKRXPolicy()) {
		t.Fatal("expected HOSE KRX ATO to be represented by a single 09:00 candle")
	}
}
