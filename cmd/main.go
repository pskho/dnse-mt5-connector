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
	"dnse-mt5-connector/internal/gmailotp"
	"dnse-mt5-connector/internal/logger"
	"dnse-mt5-connector/internal/marketdata"
	"dnse-mt5-connector/internal/risk"
	"dnse-mt5-connector/internal/service"
	"dnse-mt5-connector/internal/setup"
	"dnse-mt5-connector/internal/storage"
)

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
	positionService := service.NewPositionService(dnseClient, appLog, cfg.DNSE.AccountNo)
	orderService := service.NewOrderService(store, dnseClient, riskEngine, appLog, cfg.DNSE.AccountNo, positionService, cfg.Risk.MaxOpenPosition)
	signalService := service.NewSignalService(orderService, appLog)
	symbolCatalogService := service.NewSymbolCatalogService(appLog, "")
	historyService := marketdata.NewHistoryService(cfg.History, dnseClient, store, appLog)
	if err := historyService.Fetch(appCtx); err != nil {
		appLog.Error("history_fetch_failed", map[string]any{"error": err.Error()})
	}
	otpService := gmailotp.NewService(cfg.GmailOTP, appLog)
	if err := otpService.Start(appCtx); err != nil {
		appLog.Error("gmail_otp_start_failed", map[string]any{"error": err.Error()})
	}
	dnseClient.SetOTPFetcher(otpService)

	marketDataEngine := marketdata.NewEngine(cfg.MarketData, cfg.DNSE.APIKey, cfg.DNSE.APISecret, dnseClient, historyService, appLog)
	handler := api.NewHandler(orderService, positionService, signalService, symbolCatalogService, marketDataEngine.Profiles(), marketDataEngine, dnseClient, historyService, otpService, appLog)
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
			if !historyService.NeedsBootstrap(appCtx, cfg.History.Symbol, cfg.History.MarketType, cfg.History.Resolution) {
				appLog.Info("history_bootstrap_skipped", map[string]any{
					"reason":     "cache_present",
					"symbol":     cfg.History.Symbol,
					"marketType": cfg.History.MarketType,
					"resolution": cfg.History.Resolution,
				})
				return
			}

			appLog.Info("history_bootstrap_started", map[string]any{
				"symbol":       cfg.History.Symbol,
				"marketType":   cfg.History.MarketType,
				"resolution":   cfg.History.Resolution,
				"lookbackDays": cfg.History.InitialLookbackDays,
			})

			if _, err := historyService.BackfillBeforeToday(appCtx, cfg.History.InitialLookbackDays); err != nil {
				appLog.Error("history_bootstrap_backfill_failed", map[string]any{"error": err.Error()})
				return
			}
			if _, err := historyService.SyncToday(appCtx); err != nil {
				appLog.Error("history_bootstrap_today_failed", map[string]any{"error": err.Error()})
				return
			}
			appLog.Info("history_bootstrap_completed", map[string]any{
				"symbol": cfg.History.Symbol,
			})
		}()
	} else {
		appLog.Info("history_bootstrap_waiting_for_setup", map[string]any{
			"historyEnabled": cfg.History.Enabled,
			"hasCredentials": cfg.DNSE.HasUsableCredentials(),
		})
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
