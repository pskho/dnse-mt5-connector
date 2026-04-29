package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Host         string           `json:"host"`
	Port         int              `json:"port"`
	DatabasePath string           `json:"databasePath"`
	LogFile      string           `json:"logFile"`
	Risk         RiskConfig       `json:"risk"`
	DNSE         DNSEConfig       `json:"dnse"`
	MarketData   MarketDataConfig `json:"marketData"`
	History      HistoryConfig    `json:"history"`
	GmailOTP     GmailOTPConfig   `json:"gmailOTP"`
}

type GmailOTPConfig struct {
	Enabled             bool   `json:"enabled"`
	CredentialsFile     string `json:"credentialsFile"`
	TokenFile           string `json:"tokenFile"`
	PollIntervalSeconds int    `json:"pollIntervalSeconds"`
	EmailDomainFilter   string `json:"emailDomainFilter"`
}

type HistoryConfig struct {
	Enabled             bool   `json:"enabled"`
	Symbol              string `json:"symbol"`
	MarketType          string `json:"marketType"`
	Resolution          int    `json:"resolution"`
	InitialLookbackDays int    `json:"initialLookbackDays"`
	IncrementalSync     bool   `json:"incrementalSync"`
	FullRebuild         bool   `json:"fullRebuild"`
	MaxBatchDays        int    `json:"maxBatchDays"`
}

type RiskConfig struct {
	MaxQuantity            int `json:"maxQuantity"`
	MaxOpenPosition        int `json:"maxOpenPosition"`
	DuplicateWindowSeconds int `json:"duplicateWindowSeconds"`
}

type DNSEConfig struct {
	BaseURL      string `json:"baseUrl"`
	APIKey       string `json:"apiKey"`
	APISecret    string `json:"apiSecret"`
	SecretKey    string `json:"secretKey"`
	TradingToken string `json:"tradingToken"`
	AccountNo    string `json:"accountNo"`
	Mock         bool   `json:"mock"`
}

type MarketDataConfig struct {
	Enabled          bool     `json:"enabled"`
	Symbol           string   `json:"symbol"`
	Symbols          []string `json:"symbols"`
	BridgeAddress    string   `json:"bridgeAddress"`
	WebSocketURL     string   `json:"webSocketUrl"`
	Channels         []string `json:"channels"`
	Mock             bool     `json:"mock"`
	ReconnectSeconds int      `json:"reconnectSeconds"`
}

func Load(path string) (Config, error) {
	cfg := Config{
		Host:         "127.0.0.1",
		Port:         8080,
		DatabasePath: "data/connector.db",
		LogFile:      "logs/app.jsonl",
		Risk: RiskConfig{
			MaxQuantity:            10,
			MaxOpenPosition:        10,
			DuplicateWindowSeconds: 3,
		},
		DNSE: DNSEConfig{
			BaseURL: "https://openapi.dnse.com.vn",
			Mock:    true,
		},
		MarketData: MarketDataConfig{
			Enabled:          true,
			Symbol:           "VN30F1M",
			Symbols:          []string{"VN30F1M"},
			BridgeAddress:    "127.0.0.1:9090",
			WebSocketURL:     "wss://ws-openapi.dnse.com.vn/v1/stream?encoding=json",
			Channels:         []string{"trades.json", "quotes.json"},
			ReconnectSeconds: 5,
		},
		History: HistoryConfig{
			Enabled:             true,
			Symbol:              "VN30F1M",
			MarketType:          "DERIVATIVE",
			Resolution:          1,
			InitialLookbackDays: 365,
			IncrementalSync:     true,
			FullRebuild:         false,
			MaxBatchDays:        30,
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return loadJSONFallback("config/config.json", cfg)
		}
		return cfg, err
	}

	if strings.HasSuffix(strings.ToLower(path), ".yaml") || strings.HasSuffix(strings.ToLower(path), ".yml") {
		if err := parseSimpleYAML(data, &cfg); err != nil {
			return cfg, err
		}
		normalize(&cfg)
		return cfg, nil
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	normalize(&cfg)
	return cfg, nil
}

func (c Config) ServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func loadJSONFallback(path string, cfg Config) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	normalize(&cfg)
	return cfg, nil
}

func normalize(cfg *Config) {
	if cfg.DNSE.APISecret == "" {
		cfg.DNSE.APISecret = cfg.DNSE.SecretKey
	}
	if cfg.DNSE.SecretKey == "" {
		cfg.DNSE.SecretKey = cfg.DNSE.APISecret
	}
	if cfg.Risk.MaxQuantity <= 0 {
		cfg.Risk.MaxQuantity = 10
	}
	if cfg.Risk.MaxOpenPosition <= 0 {
		cfg.Risk.MaxOpenPosition = 10
	}
	if cfg.Risk.DuplicateWindowSeconds <= 0 {
		cfg.Risk.DuplicateWindowSeconds = 3
	}
	if cfg.MarketData.Symbol == "" {
		cfg.MarketData.Symbol = "VN30F1M"
	}
	if len(cfg.MarketData.Symbols) == 0 {
		cfg.MarketData.Symbols = []string{cfg.MarketData.Symbol}
	}
	cfg.MarketData.Symbols = normalizeSymbols(cfg.MarketData.Symbols)
	if len(cfg.MarketData.Symbols) > 0 {
		cfg.MarketData.Symbol = cfg.MarketData.Symbols[0]
	}
	if cfg.MarketData.BridgeAddress == "" {
		cfg.MarketData.BridgeAddress = "127.0.0.1:9090"
	}
	if cfg.MarketData.WebSocketURL == "" {
		cfg.MarketData.WebSocketURL = "wss://ws-openapi.dnse.com.vn/v1/stream?encoding=json"
	}
	if len(cfg.MarketData.Channels) == 0 {
		cfg.MarketData.Channels = []string{"trades.json", "quotes.json"}
	}
	if cfg.MarketData.ReconnectSeconds <= 0 {
		cfg.MarketData.ReconnectSeconds = 5
	}
	if cfg.History.Symbol == "" {
		cfg.History.Symbol = "VN30F1M"
	}
	if cfg.History.MarketType == "" {
		cfg.History.MarketType = "DERIVATIVE"
	}
	if cfg.History.Resolution <= 0 {
		cfg.History.Resolution = 1
	}
	if cfg.History.InitialLookbackDays <= 0 {
		cfg.History.InitialLookbackDays = 365
	}
	if cfg.History.MaxBatchDays <= 0 {
		cfg.History.MaxBatchDays = 30
	}
	if cfg.GmailOTP.CredentialsFile == "" {
		cfg.GmailOTP.CredentialsFile = "config/credentials.json"
	}
	if cfg.GmailOTP.TokenFile == "" {
		cfg.GmailOTP.TokenFile = "config/token.json"
	}
	if cfg.GmailOTP.PollIntervalSeconds <= 0 {
		cfg.GmailOTP.PollIntervalSeconds = 3
	}
	if cfg.GmailOTP.EmailDomainFilter == "" {
		cfg.GmailOTP.EmailDomainFilter = "dnse.com.vn"
	}
}

func parseSimpleYAML(data []byte, cfg *Config) error {
	section := ""
	for lineNo, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(stripComment(rawLine))
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, ":") {
			section = strings.TrimSuffix(line, ":")
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid config.yaml line %d", lineNo+1)
		}
		key := strings.TrimSpace(parts[0])
		value := cleanValue(parts[1])

		switch section {
		case "":
			switch key {
			case "host":
				cfg.Host = value
			case "port":
				port, err := strconv.Atoi(value)
				if err != nil {
					return fmt.Errorf("invalid port: %w", err)
				}
				cfg.Port = port
			case "database_path":
				cfg.DatabasePath = value
			case "log_file":
				cfg.LogFile = value
			}
		case "dnse":
			switch key {
			case "base_url":
				cfg.DNSE.BaseURL = value
			case "api_key":
				cfg.DNSE.APIKey = value
			case "api_secret":
				cfg.DNSE.APISecret = value
				cfg.DNSE.SecretKey = value
			case "trading_token":
				cfg.DNSE.TradingToken = value
			case "account_no":
				cfg.DNSE.AccountNo = value
			case "mock":
				enabled, err := strconv.ParseBool(value)
				if err != nil {
					return fmt.Errorf("invalid dnse.mock: %w", err)
				}
				cfg.DNSE.Mock = enabled
			}
		case "risk":
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid risk.%s: %w", key, err)
			}
			switch key {
			case "max_quantity":
				cfg.Risk.MaxQuantity = n
			case "max_open_position":
				cfg.Risk.MaxOpenPosition = n
			case "duplicate_window_seconds":
				cfg.Risk.DuplicateWindowSeconds = n
			}
		case "gmail_otp":
			switch key {
			case "enabled":
				enabled, err := strconv.ParseBool(value)
				if err != nil {
					return fmt.Errorf("invalid gmail_otp.enabled: %w", err)
				}
				cfg.GmailOTP.Enabled = enabled
			case "credentials_file":
				cfg.GmailOTP.CredentialsFile = value
			case "token_file":
				cfg.GmailOTP.TokenFile = value
			case "poll_interval_seconds":
				n, err := strconv.Atoi(value)
				if err != nil {
					return fmt.Errorf("invalid gmail_otp.poll_interval_seconds: %w", err)
				}
				cfg.GmailOTP.PollIntervalSeconds = n
			case "email_domain_filter":
				cfg.GmailOTP.EmailDomainFilter = value
			}
		case "market_data":
			switch key {
			case "enabled":
				enabled, err := strconv.ParseBool(value)
				if err != nil {
					return fmt.Errorf("invalid market_data.enabled: %w", err)
				}
				cfg.MarketData.Enabled = enabled
			case "symbol":
				cfg.MarketData.Symbol = value
			case "symbols":
				cfg.MarketData.Symbols = splitCSV(value)
			case "bridge_address":
				cfg.MarketData.BridgeAddress = value
			case "websocket_url":
				cfg.MarketData.WebSocketURL = value
			case "channels":
				cfg.MarketData.Channels = splitCSV(value)
			case "mock":
				enabled, err := strconv.ParseBool(value)
				if err != nil {
					return fmt.Errorf("invalid market_data.mock: %w", err)
				}
				cfg.MarketData.Mock = enabled
			case "reconnect_seconds":
				n, err := strconv.Atoi(value)
				if err != nil {
					return fmt.Errorf("invalid market_data.reconnect_seconds: %w", err)
				}
				cfg.MarketData.ReconnectSeconds = n
			}
		case "history":
			switch key {
			case "enabled":
				enabled, err := strconv.ParseBool(value)
				if err != nil {
					return fmt.Errorf("invalid history.enabled: %w", err)
				}
				cfg.History.Enabled = enabled
			case "symbol":
				cfg.History.Symbol = value
			case "market_type":
				cfg.History.MarketType = value
			case "resolution":
				n, err := strconv.Atoi(value)
				if err != nil {
					return fmt.Errorf("invalid history.resolution: %w", err)
				}
				cfg.History.Resolution = n
			case "initial_lookback_days":
				n, err := strconv.Atoi(value)
				if err != nil {
					return fmt.Errorf("invalid history.initial_lookback_days: %w", err)
				}
				cfg.History.InitialLookbackDays = n
			case "incremental_sync":
				enabled, err := strconv.ParseBool(value)
				if err != nil {
					return fmt.Errorf("invalid history.incremental_sync: %w", err)
				}
				cfg.History.IncrementalSync = enabled
			case "full_rebuild":
				enabled, err := strconv.ParseBool(value)
				if err != nil {
					return fmt.Errorf("invalid history.full_rebuild: %w", err)
				}
				cfg.History.FullRebuild = enabled
			case "max_batch_days":
				n, err := strconv.Atoi(value)
				if err != nil {
					return fmt.Errorf("invalid history.max_batch_days: %w", err)
				}
				cfg.History.MaxBatchDays = n
			}
		}
	}
	return nil
}

func stripComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func cleanValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return value
}

func splitCSV(value string) []string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimSpace(value[1 : len(value)-1])
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = cleanValue(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// Save writes the configuration back to the YAML file.
func (c *Config) Save(path string) error {
	yaml := fmt.Sprintf(`host: "%s"
port: %d
database_path: "%s"
log_file: "%s"

risk:
  max_quantity: %d
  max_open_position: %d
  duplicate_window_seconds: %d

dnse:
  base_url: "%s"
  api_key: "%s"
  api_secret: "%s"
  account_no: "%s"
  mock: %v

market_data:
  enabled: %v
  symbol: "%s"
  symbols: [%s]
  bridge_address: "%s"
  websocket_url: "%s"
  channels: [%s]
  mock: %v
  reconnect_seconds: %d

history:
  enabled: %v
  symbol: "%s"
  market_type: "%s"
  resolution: %d
  initial_lookback_days: %d
  incremental_sync: %v
  full_rebuild: %v
  max_batch_days: %d

gmail_otp:
  enabled: %v
  credentials_file: "%s"
  token_file: "%s"
  poll_interval_seconds: %d
  email_domain_filter: "%s"
`,
		c.Host, c.Port, c.DatabasePath, c.LogFile,
		c.Risk.MaxQuantity, c.Risk.MaxOpenPosition, c.Risk.DuplicateWindowSeconds,
		c.DNSE.BaseURL, c.DNSE.APIKey, c.DNSE.APISecret, c.DNSE.AccountNo, c.DNSE.Mock,
		c.MarketData.Enabled, c.MarketData.Symbol, strings.Join(quoteChannels(c.MarketData.Symbols), ", "), c.MarketData.BridgeAddress, c.MarketData.WebSocketURL, strings.Join(quoteChannels(c.MarketData.Channels), ", "), c.MarketData.Mock, c.MarketData.ReconnectSeconds,
		c.History.Enabled, c.History.Symbol, c.History.MarketType, c.History.Resolution, c.History.InitialLookbackDays, c.History.IncrementalSync, c.History.FullRebuild, c.History.MaxBatchDays,
		c.GmailOTP.Enabled, c.GmailOTP.CredentialsFile, c.GmailOTP.TokenFile, c.GmailOTP.PollIntervalSeconds, c.GmailOTP.EmailDomainFilter,
	)

	return os.WriteFile(path, []byte(yaml), 0644)
}

func quoteChannels(channels []string) []string {
	quoted := make([]string, len(channels))
	for i, ch := range channels {
		quoted[i] = fmt.Sprintf(`"%s"`, ch)
	}
	return quoted
}

func normalizeSymbols(symbols []string) []string {
	out := make([]string, 0, len(symbols))
	seen := make(map[string]struct{})
	for _, symbol := range symbols {
		symbol = strings.ToUpper(strings.TrimSpace(symbol))
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		out = append(out, symbol)
	}
	return out
}
