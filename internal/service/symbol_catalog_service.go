package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"dnse-mt5-connector/internal/logger"
)

const defaultDerivativeCatalogURL = "https://services-staging.entrade.com.vn/papertrade-entrade-api/derivatives"

type SymbolCatalogService struct {
	http   *http.Client
	logger *logger.FileLogger
	url    string
}

type DerivativeSymbolInfo struct {
	Symbol         string `json:"symbol"`
	Type           string `json:"type"`
	MarketPrice    float64 `json:"marketPrice"`
	CeilingPrice   float64 `json:"ceilingPrice"`
	FloorPrice     float64 `json:"floorPrice"`
	BasicPrice     float64 `json:"basicPrice"`
	ModifiedDate   string `json:"modifiedDate"`
	ExpirationDate string `json:"expirationDate"`
	ID             string `json:"id"`
}

type derivativeCatalogResponse struct {
	Total int                    `json:"total"`
	Data  []DerivativeSymbolInfo `json:"data"`
}

func NewSymbolCatalogService(appLog *logger.FileLogger, catalogURL string) *SymbolCatalogService {
	catalogURL = strings.TrimSpace(catalogURL)
	if catalogURL == "" {
		catalogURL = defaultDerivativeCatalogURL
	}
	return &SymbolCatalogService{
		http: &http.Client{Timeout: 10 * time.Second},
		logger: appLog,
		url: catalogURL,
	}
}

func (s *SymbolCatalogService) GetDerivativeSymbols(ctx context.Context) ([]DerivativeSymbolInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("failed to fetch derivative catalog")
	}
	var payload derivativeCatalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	s.logger.Info("derivative_catalog_loaded", map[string]any{
		"url":   s.url,
		"total": len(payload.Data),
	})
	return payload.Data, nil
}
