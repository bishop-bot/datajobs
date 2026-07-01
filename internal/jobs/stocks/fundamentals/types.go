package fundamentals

import (
	"time"
)

// StockMetricsTTM represents the TTM (Trailing Twelve Months) stock metrics data.
type StockMetricsTTM struct {
	Symbol              string    `json:"symbol"`
	CIK                 string    `json:"cik,omitempty"`
	Date                string    `json:"date"`
	Year                int       `json:"year"`
	Provider            string    `json:"provider"`
	Cash                *float64  `json:"cash,omitempty"`
	Current             *float64  `json:"current,omitempty"`
	Currency            string    `json:"currency,omitempty"`
	DebtToEquity        *float64  `json:"debt_to_equity,omitempty"`
	DividendPayout      *float64  `json:"dividend_payout,omitempty"`
	DividendYield       *float64  `json:"dividend_yield,omitempty"`
	EnterpriseValue     *float64  `json:"enterprise_value,omitempty"`
	FreeCashFlow        *float64  `json:"free_cash_flow,omitempty"`
	Price               *float64  `json:"price,omitempty"`
	PriceToBook         *float64  `json:"price_to_book,omitempty"`
	PriceToCashFlow     *float64  `json:"price_to_cash_flow,omitempty"`
	PriceToEarnings     *float64  `json:"price_to_earnings,omitempty"`
	PriceToFreeCashFlow *float64  `json:"price_to_free_cash_flow,omitempty"`
	PriceToSales        *float64  `json:"price_to_sales,omitempty"`
	Quick               *float64  `json:"quick,omitempty"`
	ReturnOnAssets      *float64  `json:"return_on_assets,omitempty"`
	ReturnOnEquity      *float64  `json:"return_on_equity,omitempty"`
	CreatedAt           time.Time `json:"created_at,omitempty"`
	UpdatedAt           time.Time `json:"updated_at,omitempty"`
}
