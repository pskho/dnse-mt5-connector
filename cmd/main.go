package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dnse-mt5-connector/internal/api"
	"dnse-mt5-connector/internal/config"
	"dnse-mt5-connector/internal/entrade"
	"dnse-mt5-connector/internal/gmailotp"
	"dnse-mt5-connector/internal/logger"
	"dnse-mt5-connector/internal/marketdata"
	"dnse-mt5-connector/internal/risk"
	"dnse-mt5-connector/internal/service"
	"dnse-mt5-connector/internal/setup"
	"dnse-mt5-connector/internal/storage"
	"dnse-mt5-connector/internal/telemetry"
)

const appVersion = "0.1.0-trial"

type instrumentCatalogAdapter struct {
	client *api.DNSEClient
}

func (a instrumentCatalogAdapter) GetInstruments(ctx context.Context, exchange string) ([]service.InstrumentSymbolInfo, error) {
	items, err := a.client.GetInstruments(ctx, exchange)
	if err != nil {
		return nil, err
	}
	out := make([]service.InstrumentSymbolInfo, 0, len(items))
	for _, item := range items {
		out = append(out, service.InstrumentSymbolInfo{
			Symbol:   item.Symbol,
			Exchange: item.Exchange,
			Type:     item.Type,
		})
	}
	return out, nil
}

func (a instrumentCatalogAdapter) GetTickers(ctx context.Context, symbol string) ([]service.TickerMetadataInfo, error) {
	items, err := a.client.GetTickers(ctx, symbol)
	if err != nil {
		return nil, err
	}
	out := make([]service.TickerMetadataInfo, 0, len(items))
	for _, item := range items {
		out = append(out, service.TickerMetadataInfo{
			Symbol:      item.Symbol,
			FeedSymbol:  item.FeedSymbol,
			Exchange:    item.Exchange,
			Type:        item.Type,
			BoardID:     item.BoardID,
			Name:        item.Name,
			Description: item.Description,
			RawJSON:     item.RawJSON,
		})
	}
	return out, nil
}

func main() {
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	if err := setup.EnsureDirectories(); err != nil {
		log.Fatalf("failed to ensure directories: %v", err)
	}

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	appLog, err := logger.NewFileLogger(cfg.LogFile)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer appLog.Close()
	telemetryClient := telemetry.NewClient(cfg.Telemetry, appLog)
	telemetryClient.Track(appCtx, "app_start", map[string]any{
		"app_version":           appVersion,
		"provider":              tradingProviderName(cfg),
		"symbol_count":          len(cfg.MarketData.Symbols),
		"default_target_count":  len(cfg.Entrade.DefaultAccountNos),
		"entrade_profile_count": len(cfg.Entrade.Accounts),
		"primary_symbol":        cfg.MarketData.Symbol,
		"history_enabled":       cfg.History.Enabled,
		"market_data_enabled":   cfg.MarketData.Enabled,
		"gmail_otp_enabled":     cfg.GmailOTP.Enabled,
		"mock_mode":             cfg.DNSE.Mock || cfg.Entrade.Mock || cfg.MarketData.Mock,
	})

	store, err := storage.NewSQLiteStore(cfg.DatabasePath)
	if err != nil {
		appLog.Error("storage_init_failed", map[string]any{"error": err.Error()})
		log.Fatalf("init storage: %v", err)
	}
	defer store.Close()

	if err := store.Migrate(context.Background()); err != nil {
		appLog.Error("storage_migrate_failed", map[string]any{"error": err.Error()})
		log.Fatalf("migrate storage: %v", err)
	}

	dnseClient := api.NewDNSEClient(cfg.DNSE, appLog, store)
	dnseClient.LoadPersistedToken(context.Background())
	riskEngine := risk.NewEngine(cfg.Risk, store)
	tradingClient := service.DNSEClient(dnseClient)
	positionClient := service.PositionClient(dnseClient)
	defaultTradingAccountNo := cfg.DNSE.AccountNo
	defaultPositionAccountNo := cfg.DNSE.AccountNo
	if cfg.Entrade.Enabled {
		entradeClient := entrade.NewClient(cfg.Entrade, appLog)
		tradingClient = entradeClient
		positionClient = entradeClient
		defaultTradingAccountNo = strings.Join(cfg.Entrade.DefaultAccountNos, ",")
		if defaultTradingAccountNo == "" {
			defaultTradingAccountNo = cfg.Entrade.AccountNo
		}
		defaultPositionAccountNo = ""
		if len(cfg.Entrade.DefaultAccountNos) > 0 {
			defaultPositionAccountNo = cfg.Entrade.DefaultAccountNos[0]
		}
		if defaultPositionAccountNo == "" {
			defaultPositionAccountNo = cfg.Entrade.AccountNo
		}
		appLog.Info("trading_provider_selected", map[string]any{
			"provider":        "entrade",
			"environment":     cfg.Entrade.Environment,
			"defaultAccounts": cfg.Entrade.DefaultAccountNos,
		})
	} else {
		appLog.Info("trading_provider_selected", map[string]any{"provider": "dnse"})
	}
	positionService := service.NewPositionService(positionClient, appLog, defaultPositionAccountNo)
	orderService := service.NewOrderService(store, tradingClient, riskEngine, appLog, defaultTradingAccountNo, positionService, cfg.Risk.MaxOpenPosition)
	signalService := service.NewSignalService(orderService, appLog)
	symbolCatalogService := service.NewSymbolCatalogService(appLog, "", instrumentCatalogAdapter{client: dnseClient}, instrumentCatalogAdapter{client: dnseClient}, store)
	historyService := marketdata.NewHistoryService(cfg.History, dnseClient, store, appLog)
	if err := historyService.Fetch(appCtx); err != nil {
		appLog.Error("history_fetch_failed", map[string]any{"error": err.Error()})
	}
	otpService := gmailotp.NewService(cfg.GmailOTP, appLog)
	if err := otpService.Start(appCtx); err != nil {
		appLog.Error("gmail_otp_start_failed", map[string]any{"error": err.Error()})
	}
	dnseClient.SetOTPFetcher(otpService)

	marketDataEngine := marketdata.NewEngine(cfg.MarketData, cfg.DNSE.APIKey, cfg.DNSE.APISecret, dnseClient, symbolCatalogService, historyService, appLog)
	handler := api.NewHandler(orderService, positionService, signalService, symbolCatalogService, marketDataEngine.Profiles(), marketDataEngine, dnseClient, historyService, otpService, appLog, telemetryClient)
	marketDataEngine.Start(appCtx)

	server := &http.Server{
		Addr:              cfg.ServerAddress(),
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		appLog.Info("server_starting", map[string]any{"address": server.Addr})
		log.Printf("DNSE MT5 Connector listening on http://%s", server.Addr)
		log.Printf("Market data bridge listening on tcp://%s for primary %s", cfg.MarketData.BridgeAddress, cfg.MarketData.Symbol)
		log.Printf("Monitoring symbols: %s", strings.Join(cfg.MarketData.Symbols, ", "))
		log.Printf("Mode: manual. Use /status for health, /signal for MT5 signals, and /confirm for user-approved orders.")

		go func() {
			time.Sleep(1 * time.Second)
			exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://127.0.0.1:8080/setup").Start()
		}()

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLog.Error("server_failed", map[string]any{"error": err.Error()})
			log.Fatalf("server failed: %v", err)
		}
	}()

	if cfg.History.Enabled && cfg.DNSE.HasUsableCredentials() {
		go func() {
			time.Sleep(2 * time.Second)
			profiles := marketDataEngine.Profiles()
			primary := strings.ToUpper(strings.TrimSpace(cfg.MarketData.Symbol))
			ordered := make([]marketdata.SymbolProfile, 0, len(profiles))
			for _, profile := range profiles {
				if strings.EqualFold(profile.Symbol, primary) {
					continue
				}
				ordered = append(ordered, profile)
			}
			for _, profile := range profiles {
				if strings.EqualFold(profile.Symbol, primary) {
					ordered = append(ordered, profile)
					break
				}
			}

			for _, profile := range ordered {
				if !historyService.NeedsBootstrap(appCtx, profile.Symbol, profile.MarketType, profile.Resolution) {
					appLog.Info("history_bootstrap_skipped", map[string]any{
						"reason":     "cache_present",
						"symbol":     profile.Symbol,
						"marketType": profile.MarketType,
						"resolution": profile.Resolution,
					})
					continue
				}

				appLog.Info("history_bootstrap_started", map[string]any{
					"symbol":       profile.Symbol,
					"marketType":   profile.MarketType,
					"resolution":   profile.Resolution,
					"lookbackDays": cfg.History.InitialLookbackDays,
				})

				if _, err := historyService.SyncWithOptions(appCtx, marketdata.SyncOptions{
					ForceFull:    true,
					BeforeToday:  true,
					LookbackDays: cfg.History.InitialLookbackDays,
					Symbol:       profile.Symbol,
					MarketType:   profile.MarketType,
					Resolution:   profile.Resolution,
				}); err != nil {
					appLog.Error("history_bootstrap_backfill_failed", map[string]any{
						"symbol": profile.Symbol,
						"error":  err.Error(),
					})
					continue
				}

				if _, err := historyService.SyncWithOptions(appCtx, marketdata.SyncOptions{
					TodayOnly:  true,
					Symbol:     profile.Symbol,
					MarketType: profile.MarketType,
					Resolution: profile.Resolution,
				}); err != nil {
					appLog.Error("history_bootstrap_today_failed", map[string]any{
						"symbol": profile.Symbol,
						"error":  err.Error(),
					})
					continue
				}

				appLog.Info("history_bootstrap_completed", map[string]any{
					"symbol":     profile.Symbol,
					"marketType": profile.MarketType,
					"resolution": profile.Resolution,
				})
			}
		}()
	} else {
		appLog.Info("history_bootstrap_waiting_for_setup", map[string]any{
			"historyEnabled": cfg.History.Enabled,
			"hasCredentials": cfg.DNSE.HasUsableCredentials(),
		})
	}

	if cfg.DNSE.HasUsableCredentials() && cfg.GmailOTP.Enabled {
		go maintainDNSETradingToken(appCtx, dnseClient, appLog)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	appCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	appLog.Info("server_stopping", nil)
	if err := server.Shutdown(ctx); err != nil {
		appLog.Error("server_shutdown_failed", map[string]any{"error": err.Error()})
	}
}

func maintainDNSETradingToken(ctx context.Context, client *api.DNSEClient, appLog *logger.FileLogger) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		allowed, minValidity, reason := dnseTokenRefreshPolicy(time.Now())
		if !allowed {
			status := client.TradingTokenStatus()
			appLog.Info("dnse_trading_token_refresh_skipped", map[string]any{
				"reason":           reason,
				"valid":            status.Valid,
				"remainingSeconds": status.RemainingSeconds,
			})
		} else {
			if err := client.EnsureTradingToken(ctx, minValidity); err != nil {
				appLog.Error("dnse_trading_token_auto_refresh_failed", map[string]any{"error": err.Error(), "reason": reason})
			} else {
				status := client.TradingTokenStatus()
				appLog.Info("dnse_trading_token_ready", map[string]any{
					"reason":           reason,
					"valid":            status.Valid,
					"expiresAt":        status.ExpiresAt.UTC().Format(time.RFC3339),
					"remainingSeconds": status.RemainingSeconds,
				})
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func dnseTokenRefreshPolicy(now time.Time) (bool, time.Duration, string) {
	local, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		local = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	t := now.In(local)
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return false, 0, "outside_weekday"
	}
	minute := t.Hour()*60 + t.Minute()
	openRefresh := 8 * 60
	closeRefresh := 15 * 60
	if minute < openRefresh || minute > closeRefresh {
		return false, 0, "outside_refresh_window_0800_1500"
	}
	if minute >= openRefresh && minute < openRefresh+10 {
		return true, 8 * time.Hour, "daily_0800_refresh"
	}
	return true, 30 * time.Minute, "trading_session_refresh"
}

func tradingProviderName(cfg config.Config) string {
	if cfg.Entrade.Enabled {
		return "entrade"
	}
	return "dnse"
}
