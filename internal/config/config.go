package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var defaultTrackedSymbols = []string{
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

type Config struct {
	Host         string           `json:"host"`
	Port         int              `json:"port"`
	DatabasePath string           `json:"databasePath"`
	LogFile      string           `json:"logFile"`
	Risk         RiskConfig       `json:"risk"`
	DNSE         DNSEConfig       `json:"dnse"`
	Entrade      EntradeConfig    `json:"entrade"`
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

type EntradeConfig struct {
	Enabled           bool                   `json:"enabled"`
	Environment       string                 `json:"environment"`
	AuthURL           string                 `json:"authUrl"`
	BaseURL           string                 `json:"baseUrl"`
	PaperBaseURL      string                 `json:"paperBaseUrl"`
	Username          string                 `json:"username"`
	Password          string                 `json:"password"`
	InvestorID        string                 `json:"investorId"`
	AccountNo         string                 `json:"accountNo"`
	DefaultAccountNos []string               `json:"defaultAccountNos"`
	Accounts          []EntradeAccountConfig `json:"accounts"`
	TradingToken      string                 `json:"tradingToken"`
	Mock              bool                   `json:"mock"`
}

type EntradeAccountConfig struct {
	ID           string `json:"id"`
	Environment  string `json:"environment"`
	Username     string `json:"username"`
	Password     string `json:"password,omitempty"`
	InvestorID   string `json:"investorId,omitempty"`
	AccountNo    string `json:"accountNo,omitempty"`
	TradingToken string `json:"tradingToken,omitempty"`
	Enabled      bool   `json:"enabled"`
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
		Entrade: EntradeConfig{
			Environment:       "paper",
			AuthURL:           "https://services.entrade.com.vn/entrade-api/v2/auth",
			BaseURL:           "https://services.entrade.com.vn/entrade-api",
			PaperBaseURL:      "https://services.entrade.com.vn/papertrade-entrade-api",
			DefaultAccountNos: []string{"ENTRADE_DEMO"},
		},
		MarketData: MarketDataConfig{
			Enabled:          true,
			Symbol:           "VN30F1M",
			Symbols:          append([]string(nil), defaultTrackedSymbols...),
			BridgeAddress:    "127.0.0.1:9090",
			WebSocketURL:     "wss://ws-openapi.dnse.com.vn/v1/stream?encoding=json",
			Channels:         []string{"tick.G1.json", "top_price.G1.json", "ohlc.1.json", "ohlc_closed.1.json"},
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

func (c DNSEConfig) HasUsableCredentials() bool {
	return !isPlaceholder(c.APIKey) && !isPlaceholder(c.APISecret)
}

func isPlaceholder(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	upper := strings.ToUpper(value)
	return strings.Contains(upper, "PASTE_DNSE_API_KEY_HERE") ||
		strings.Contains(upper, "PASTE_DNSE_API_SECRET_HERE") ||
		strings.Contains(upper, "PASTE_ACCOUNT_NO_HERE")
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
	if cfg.Entrade.Environment == "" {
		cfg.Entrade.Environment = "paper"
	}
	if cfg.Entrade.AuthURL == "" {
		cfg.Entrade.AuthURL = "https://services.entrade.com.vn/entrade-api/v2/auth"
	}
	if cfg.Entrade.BaseURL == "" {
		cfg.Entrade.BaseURL = "https://services.entrade.com.vn/entrade-api"
	}
	if cfg.Entrade.PaperBaseURL == "" {
		cfg.Entrade.PaperBaseURL = "https://services.entrade.com.vn/papertrade-entrade-api"
	}
	cfg.Entrade.DefaultAccountNos = normalizeAccountNos(cfg.Entrade.DefaultAccountNos)
	if len(cfg.Entrade.DefaultAccountNos) == 0 && strings.TrimSpace(cfg.Entrade.AccountNo) != "" {
		cfg.Entrade.DefaultAccountNos = normalizeAccountNos([]string{cfg.Entrade.AccountNo})
	}
	cfg.Entrade.Accounts = normalizeEntradeAccounts(cfg.Entrade)
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
		if strings.TrimSpace(cfg.MarketData.Symbol) != "" {
			cfg.MarketData.Symbols = []string{cfg.MarketData.Symbol}
		} else {
			cfg.MarketData.Symbols = append([]string(nil), defaultTrackedSymbols...)
		}
	}
	cfg.MarketData.Symbols = normalizeSymbols(cfg.MarketData.Symbols)
	if len(cfg.MarketData.Symbols) > 0 {
		cfg.MarketData.Symbol = cfg.MarketData.Symbols[0]
	} else {
		cfg.MarketData.Symbol = "VN30F1M"
		cfg.MarketData.Symbols = append([]string(nil), defaultTrackedSymbols...)
	}
	if cfg.MarketData.BridgeAddress == "" {
		cfg.MarketData.BridgeAddress = "127.0.0.1:9090"
	}
	if cfg.MarketData.WebSocketURL == "" {
		cfg.MarketData.WebSocketURL = "wss://ws-openapi.dnse.com.vn/v1/stream?encoding=json"
	}
	if len(cfg.MarketData.Channels) == 0 {
		cfg.MarketData.Channels = []string{"tick.G1.json", "top_price.G1.json", "ohlc.1.json", "ohlc_closed.1.json"}
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
		case "entrade":
			switch key {
			case "enabled":
				enabled, err := strconv.ParseBool(value)
				if err != nil {
					return fmt.Errorf("invalid entrade.enabled: %w", err)
				}
				cfg.Entrade.Enabled = enabled
			case "environment":
				cfg.Entrade.Environment = value
			case "auth_url":
				cfg.Entrade.AuthURL = value
			case "base_url":
				cfg.Entrade.BaseURL = value
			case "paper_base_url":
				cfg.Entrade.PaperBaseURL = value
			case "username":
				cfg.Entrade.Username = value
			case "password":
				cfg.Entrade.Password = value
			case "investor_id":
				cfg.Entrade.InvestorID = value
			case "account_no":
				cfg.Entrade.AccountNo = value
			case "default_accounts":
				cfg.Entrade.DefaultAccountNos = splitCSV(value)
			case "account_profiles":
				cfg.Entrade.Accounts = parseEntradeAccountProfiles(value)
			case "trading_token":
				cfg.Entrade.TradingToken = value
			case "mock":
				enabled, err := strconv.ParseBool(value)
				if err != nil {
					return fmt.Errorf("invalid entrade.mock: %w", err)
				}
				cfg.Entrade.Mock = enabled
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
  trading_token: "%s"
  account_no: "%s"
  mock: %v

entrade:
  enabled: %v
  environment: "%s"
  auth_url: "%s"
  base_url: "%s"
  paper_base_url: "%s"
  username: "%s"
  password: "%s"
  investor_id: "%s"
  account_no: "%s"
  default_accounts: [%s]
  account_profiles: "%s"
  trading_token: "%s"
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
		c.DNSE.BaseURL, c.DNSE.APIKey, c.DNSE.APISecret, c.DNSE.TradingToken, c.DNSE.AccountNo, c.DNSE.Mock,
		c.Entrade.Enabled, c.Entrade.Environment, c.Entrade.AuthURL, c.Entrade.BaseURL, c.Entrade.PaperBaseURL,
		c.Entrade.Username, c.Entrade.Password, c.Entrade.InvestorID, c.Entrade.AccountNo,
		strings.Join(quoteChannels(c.Entrade.DefaultAccountNos), ", "), formatEntradeAccountProfiles(c.Entrade.Accounts),
		c.Entrade.TradingToken, c.Entrade.Mock,
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
		if strings.HasPrefix(symbol, "VN100F") {
			symbol = "V100F" + strings.TrimPrefix(symbol, "VN100F")
		}
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

func normalizeAccountNos(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeEntradeAccounts(cfg EntradeConfig) []EntradeAccountConfig {
	accounts := make([]EntradeAccountConfig, 0, len(cfg.Accounts)+1)
	seen := map[string]struct{}{}
	add := func(account EntradeAccountConfig) {
		account.ID = strings.ToUpper(strings.TrimSpace(account.ID))
		if account.ID == "" {
			return
		}
		if _, ok := seen[account.ID]; ok {
			return
		}
		seen[account.ID] = struct{}{}
		account.Environment = strings.ToLower(strings.TrimSpace(account.Environment))
		if account.Environment == "" {
			account.Environment = "paper"
		}
		if account.Username == "" {
			account.Username = cfg.Username
		}
		if account.Password == "" {
			account.Password = cfg.Password
		}
		if account.InvestorID == "" {
			account.InvestorID = cfg.InvestorID
		}
		if account.AccountNo == "" {
			account.AccountNo = cfg.AccountNo
		}
		if account.TradingToken == "" {
			account.TradingToken = cfg.TradingToken
		}
		accounts = append(accounts, account)
	}
	for _, account := range cfg.Accounts {
		if account.ID != "" {
			add(account)
		}
	}
	if len(accounts) == 0 && strings.TrimSpace(cfg.Username) != "" {
		add(EntradeAccountConfig{ID: "ENTRADE_DEMO", Environment: "paper", Enabled: true})
	}
	return accounts
}

func parseEntradeAccountProfiles(value string) []EntradeAccountConfig {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	rows := strings.Split(value, ";")
	out := make([]EntradeAccountConfig, 0, len(rows))
	for _, row := range rows {
		cols := strings.Split(row, "|")
		if len(cols) < 4 {
			continue
		}
		account := EntradeAccountConfig{
			ID:          strings.ToUpper(strings.TrimSpace(cols[0])),
			Environment: strings.ToLower(strings.TrimSpace(cols[1])),
			Username:    strings.TrimSpace(cols[2]),
			Password:    strings.TrimSpace(cols[3]),
			Enabled:     true,
		}
		if len(cols) > 4 {
			account.InvestorID = strings.TrimSpace(cols[4])
		}
		if len(cols) > 5 {
			account.AccountNo = strings.TrimSpace(cols[5])
		}
		if len(cols) > 6 {
			account.TradingToken = strings.TrimSpace(cols[6])
		}
		if len(cols) > 7 {
			account.Enabled, _ = strconv.ParseBool(strings.TrimSpace(cols[7]))
		}
		out = append(out, account)
	}
	return out
}

func formatEntradeAccountProfiles(accounts []EntradeAccountConfig) string {
	rows := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if !account.Enabled || strings.TrimSpace(account.ID) == "" {
			continue
		}
		rows = append(rows, strings.Join([]string{
			strings.ToUpper(strings.TrimSpace(account.ID)),
			strings.ToLower(strings.TrimSpace(account.Environment)),
			strings.ReplaceAll(strings.TrimSpace(account.Username), "|", ""),
			strings.ReplaceAll(strings.TrimSpace(account.Password), "|", ""),
			strings.ReplaceAll(strings.TrimSpace(account.InvestorID), "|", ""),
			strings.ReplaceAll(strings.TrimSpace(account.AccountNo), "|", ""),
			strings.ReplaceAll(strings.TrimSpace(account.TradingToken), "|", ""),
			strconv.FormatBool(account.Enabled),
		}, "|"))
	}
	return strings.ReplaceAll(strings.Join(rows, ";"), `"`, `\"`)
}
