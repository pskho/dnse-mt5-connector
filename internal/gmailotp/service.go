package gmailotp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"dnse-mt5-connector/internal/config"
	"dnse-mt5-connector/internal/logger"
)

type CachedOTP struct {
	Code      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type Service struct {
	cfg       config.GmailOTPConfig
	logger    *logger.FileLogger
	srv       *gmail.Service
	mu        sync.RWMutex
	latest    CachedOTP
	processed map[string]bool
	otpRegex  *regexp.Regexp
}

func NewService(cfg config.GmailOTPConfig, appLog *logger.FileLogger) *Service {
	return &Service{
		cfg:       cfg,
		logger:    appLog,
		processed: make(map[string]bool),
		otpRegex:  regexp.MustCompile(`\b\d{6}\b`),
	}
}

func (s *Service) Start(ctx context.Context) error {
	if !s.cfg.Enabled {
		return nil
	}

	client, err := s.getOAuthClient(ctx)
	if err != nil {
		s.logger.Error("gmail_oauth_failed", map[string]any{"error": err.Error()})
		return err
	}

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		s.logger.Error("gmail_service_failed", map[string]any{"error": err.Error()})
		return err
	}
	s.srv = srv

	go s.pollLoop(ctx)
	return nil
}

func (s *Service) GetLatestOTP() (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.latest.Code == "" {
		return "", false
	}
	if time.Now().After(s.latest.ExpiresAt) {
		return "", false
	}
	return s.latest.Code, true
}

func (s *Service) pollLoop(ctx context.Context) {
	s.logger.Info("gmail_poll_started", map[string]any{"interval": s.cfg.PollIntervalSeconds})
	ticker := time.NewTicker(time.Duration(s.cfg.PollIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("gmail_poll_stopped", nil)
			return
		case <-ticker.C:
			s.checkEmails()
		}
	}
}

func (s *Service) checkEmails() {
	query := fmt.Sprintf(`from:%s is:unread subject:(OTP OR Verification OR "mã xác thực") newer_than:1d`, s.cfg.EmailDomainFilter)
	user := "me"

	res, err := s.srv.Users.Messages.List(user).Q(query).MaxResults(5).Do()
	if err != nil {
		s.logger.Error("gmail_list_failed", map[string]any{"error": err.Error()})
		return
	}

	if len(res.Messages) == 0 {
		return
	}

	for _, msgItem := range res.Messages {
		s.mu.RLock()
		isProcessed := s.processed[msgItem.Id]
		s.mu.RUnlock()
		if isProcessed {
			continue
		}

		msg, err := s.srv.Users.Messages.Get(user, msgItem.Id).Format("full").Do()
		if err != nil {
			s.logger.Error("gmail_get_msg_failed", map[string]any{"id": msgItem.Id, "error": err.Error()})
			continue
		}

		// internalDate is in milliseconds
		msgTime := time.Unix(0, msg.InternalDate*int64(time.Millisecond))
		if time.Since(msgTime) > 2*time.Minute {
			s.markProcessed(msgItem.Id)
			s.logger.Info("gmail_msg_ignored_old", map[string]any{"id": msgItem.Id, "ageSeconds": time.Since(msgTime).Seconds()})
			continue
		}

		bodyText := s.extractBody(msg.Payload)
		otp := s.parseOTP(bodyText)

		if otp != "" {
			s.mu.Lock()
			s.latest = CachedOTP{
				Code:      otp,
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(60 * time.Second),
			}
			s.processed[msgItem.Id] = true
			s.mu.Unlock()

			s.logger.Info("gmail_otp_extracted", map[string]any{"id": msgItem.Id, "otp": "***" + otp[3:]})
			s.markAsRead(user, msgItem.Id)
		} else {
			s.markProcessed(msgItem.Id)
			s.logger.Info("gmail_otp_not_found", map[string]any{"id": msgItem.Id})
		}
	}
}

func (s *Service) markProcessed(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processed[id] = true
}

func (s *Service) markAsRead(user, msgId string) {
	modReq := &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{"UNREAD"},
	}
	_, err := s.srv.Users.Messages.Modify(user, msgId, modReq).Do()
	if err != nil {
		s.logger.Error("gmail_mark_read_failed", map[string]any{"id": msgId, "error": err.Error()})
	}
}

func (s *Service) extractBody(part *gmail.MessagePart) string {
	if part == nil {
		return ""
	}
	if part.MimeType == "text/plain" || part.MimeType == "text/html" {
		data, err := base64.URLEncoding.DecodeString(part.Body.Data)
		if err != nil {
			return ""
		}
		return string(data)
	}

	var sb strings.Builder
	for _, subPart := range part.Parts {
		sb.WriteString(s.extractBody(subPart))
		sb.WriteString(" ")
	}
	return sb.String()
}

func (s *Service) parseOTP(body string) string {
	matches := s.otpRegex.FindAllString(body, -1)
	for _, match := range matches {
		if len(match) == 6 {
			return match
		}
	}
	return ""
}

// OAuth2 Flow
func (s *Service) getOAuthClient(ctx context.Context) (*http.Client, error) {
	b, err := os.ReadFile(s.cfg.CredentialsFile)
	if err != nil {
		s.logger.Info("gmail_auth_instructions", map[string]any{
			"message": "credentials.json is missing. Please create a Google Cloud Project, enable Gmail API, download OAuth 2.0 Client credentials, and save as " + s.cfg.CredentialsFile,
		})
		return nil, fmt.Errorf("unable to read credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope, gmail.GmailModifyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials file to config: %w", err)
	}

	tok, err := s.tokenFromFile(s.cfg.TokenFile)
	if err != nil {
		tok = s.getTokenFromWeb(config)
		s.saveToken(s.cfg.TokenFile, tok)
	}
	return config.Client(ctx, tok), nil
}

func (s *Service) getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("\n=======================================================\n")
	fmt.Printf("GMAIL OAUTH2 REQUIRED for OTP Auto-Fetch\n")
	fmt.Printf("Go to the following link in your browser then type the authorization code:\n%v\n", authURL)
	fmt.Printf("=======================================================\n")
	fmt.Printf("Enter authorization code: ")

	var input string
	if _, err := fmt.Scan(&input); err != nil {
		s.logger.Error("gmail_auth_code_read_failed", map[string]any{"error": err.Error()})
		return nil
	}

	authCode := input
	if strings.HasPrefix(input, "http") {
		if u, err := url.Parse(input); err == nil {
			if code := u.Query().Get("code"); code != "" {
				authCode = code
			}
		}
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		s.logger.Error("gmail_token_exchange_failed", map[string]any{"error": err.Error()})
		return nil
	}
	return tok
}

func (s *Service) tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func (s *Service) saveToken(path string, token *oauth2.Token) {
	if token == nil {
		return
	}
	s.logger.Info("gmail_token_saving", map[string]any{"path": path})
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		s.logger.Error("gmail_token_save_failed", map[string]any{"error": err.Error()})
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
