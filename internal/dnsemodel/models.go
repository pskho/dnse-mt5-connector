package dnsemodel

type Account struct {
	AccountNo               string `json:"accountNo"`
	DerivativeAccountStatus string `json:"derivativeAccountStatus"`
}

type PlaceOrderRequest struct {
	ClientOrderID string  `json:"clientOrderId,omitempty"`
	AccountNo     string  `json:"accountNo"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	Quantity      int     `json:"quantity"`
	Price         float64 `json:"price,omitempty"`
	OrderType     string  `json:"orderType"`
	LoanPackageID *int    `json:"loanPackageId,omitempty"`
	MarketType    string  `json:"-"`
	OrderCategory string  `json:"-"`
}

type PlaceOrderResponse struct {
	OrderID     string `json:"orderId"`
	Status      string `json:"status"`
	RawResponse string `json:"-"`
}

type OrderStatus struct {
	OrderID           string `json:"orderId"`
	Status            string `json:"status"`
	FilledQuantity    int    `json:"filledQuantity"`
	RemainingQuantity int    `json:"remainingQuantity"`
	RawResponse       string `json:"-"`
}

type CancelOrderResponse struct {
	Success           bool   `json:"success"`
	OrderID           string `json:"orderId"`
	Status            string `json:"status,omitempty"`
	FilledQuantity    int    `json:"filledQuantity,omitempty"`
	RemainingQuantity int    `json:"remainingQuantity,omitempty"`
	RawResponse       string `json:"-"`
}

type LoanPackage struct {
	ID              int     `json:"id"`
	Name            string  `json:"name,omitempty"`
	InterestRate    float64 `json:"interestRate,omitempty"`
	InitialRate     float64 `json:"initialRate,omitempty"`
	MaintenanceRate float64 `json:"maintenanceRate,omitempty"`
	LiquidRate      float64 `json:"liquidRate,omitempty"`
	Type            string  `json:"type,omitempty"`
}

type PPSE struct {
	Price    float64 `json:"price,omitempty"`
	QmaxBuy  int     `json:"qmaxBuy,omitempty"`
	QmaxSell int     `json:"qmaxSell,omitempty"`
}

type Position struct {
	ID       string `json:"id,omitempty"`
	Symbol   string `json:"symbol"`
	Side     string `json:"side,omitempty"`
	Quantity int    `json:"quantity"`
}

type CloseDealResponse struct {
	Success     bool   `json:"success"`
	DealID      string `json:"dealId,omitempty"`
	OrderID     string `json:"orderId,omitempty"`
	Status      string `json:"status,omitempty"`
	AccountNo   string `json:"accountNo,omitempty"`
	RawResponse string `json:"-"`
}
