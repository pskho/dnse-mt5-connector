package service

type OrderRequest struct {
	ClientOrderID string  `json:"clientOrderId"`
	AccountNo     string  `json:"accountNo"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	Quantity      int     `json:"quantity"`
	Price         float64 `json:"price"`
	OrderType     string  `json:"orderType"`
	LoanPackageID *int    `json:"loanPackageId,omitempty"`
	MarketType    string  `json:"marketType,omitempty"`
	OrderCategory string  `json:"orderCategory,omitempty"`
}

type OrderResponse struct {
	Success bool   `json:"success"`
	OrderID  string `json:"orderId,omitempty"`
	Status   string `json:"status,omitempty"`
	Message  string `json:"message,omitempty"`
}

type Account struct {
	AccountNo string `json:"accountNo"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type,omitempty"`
}

type CancelRequest struct {
	AccountNo      string `json:"accountNo,omitempty"`
	OrderID        string `json:"orderId"`
	MarketType     string `json:"marketType,omitempty"`
	OrderCategory  string `json:"orderCategory,omitempty"`
}

type CancelResponse struct {
	Success           bool   `json:"success"`
	OrderID           string `json:"orderId"`
	Status            string `json:"status,omitempty"`
	FilledQuantity    int    `json:"filledQuantity,omitempty"`
	RemainingQuantity int    `json:"remainingQuantity,omitempty"`
}

type OrderStatusResponse struct {
	OrderID           string `json:"orderId"`
	Status            string `json:"status"`
	FilledQuantity    int    `json:"filledQuantity"`
	RemainingQuantity int    `json:"remainingQuantity"`
	Stale             bool   `json:"stale,omitempty"`
	Warning           string `json:"warning,omitempty"`
}

type Position struct {
	Symbol      string `json:"symbol"`
	Direction   string `json:"direction"`
	NetQuantity int    `json:"netQuantity"`
	LongQuantity int    `json:"longQuantity"`
	ShortQuantity int   `json:"shortQuantity"`
}
