package service

import (
	"context"
	"time"
)

type SelfTestResult struct {
	Time    string          `json:"time"`
	Passed bool            `json:"passed"`
	Checks []SelfTestCheck `json:"checks"`
}

type SelfTestCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

func NewSelfTestResult() SelfTestResult {
	return SelfTestResult{
		Time:    time.Now().UTC().Format(time.RFC3339),
		Passed: true,
		Checks: make([]SelfTestCheck, 0),
	}
}

func (r *SelfTestResult) Add(name string, passed bool, message string) {
	r.Checks = append(r.Checks, SelfTestCheck{Name: name, Passed: passed, Message: message})
	if !passed {
		r.Passed = false
	}
}

func (s *OrderService) SelfTest(ctx context.Context) SelfTestResult {
	result := NewSelfTestResult()

	accounts, err := s.dnse.GetAccounts(ctx)
	if err != nil {
		result.Add("dnse_accounts", false, err.Error())
		return result
	}
	if len(accounts) == 0 {
		result.Add("dnse_accounts", false, "DNSE returned no accounts")
		return result
	}
	result.Add("dnse_accounts", true, "accountNo="+accounts[0].AccountNo)

	accountNo := accounts[0].AccountNo
	symbol := "VN30F1M"
	marketType := "DERIVATIVE"
	packages, err := s.dnse.GetLoanPackages(ctx, accountNo, symbol, marketType)
	if err != nil {
		result.Add("loan_packages_derivative", false, err.Error())
	} else if len(packages) == 0 {
		result.Add("loan_packages_derivative", false, "DNSE returned no derivative loan packages")
	} else {
		result.Add("loan_packages_derivative", true, "firstPackageId="+itoa(packages[0].ID))
	}

	_, _, err = s.validateOrder(OrderRequest{
		ClientOrderID: "self-test-validation",
		AccountNo:     accountNo,
		Symbol:        symbol,
		Side:          "BUY",
		Quantity:      1,
		Price:         0,
		OrderType:     "MTL",
		MarketType:    marketType,
	})
	if err != nil {
		result.Add("order_validation", false, err.Error())
	} else {
		result.Add("order_validation", true, "DERIVATIVE MTL validation passed")
	}

	return result
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 12)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
