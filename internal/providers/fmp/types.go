package fmp

// FinancialRatiosResponse represents the API response for the financial ratios endpoint.
type FinancialRatiosResponse struct {
	Symbol                    string  `json:"symbol"`
	Date                      string  `json:"date"`
	// Profitability Ratios
	GrossProfitMargin         *float64 `json:"grossProfitMargin"`
	OperatingProfitMargin     *float64 `json:"operatingProfitMargin"`
	NetProfitMargin           *float64 `json:"netProfitMargin"`
	ReturnOnAssets            *float64 `json:"returnOnAssets"`
	ReturnOnEquity            *float64 `json:"returnOnEquity"`
	ReturnOnCapitalEmployed   *float64 `json:"returnOnCapitalEmployed"`
	ROIC                      *float64 `json:"ROIC"`
	// Liquidity Ratios
	CurrentRatio              *float64 `json:"currentRatio"`
	QuickRatio                *float64 `json:"quickRatio"`
	CashRatio                 *float64 `json:"cashRatio"`
	WorkingCapital            *float64 `json:"workingCapital"`
	// Efficiency Ratios
	AssetTurnover            *float64 `json:"assetTurnover"`
	InventoryTurnoverRatio   *float64 `json:"inventoryTurnoverRatio"`
	ReceivablesTurnover     *float64 `json:"receivablesTurnover"`
	PayablesTurnover        *float64 `json:"payablesTurnover"`
	DaysSalesOutstanding    *float64 `json:"daysSalesOutstanding"`
	DaysInventoryOutstanding *float64 `json:"daysInventoryOutstanding"`
	OperatingCycle           *float64 `json:"operatingCycle"`
	DaysPayablesOutstanding  *float64 `json:"daysPayablesOutstanding"`
	CashConversionCycle      *float64 `json:"cashConversionCycle"`
	// Leverage Ratios
	DebtRatio                *float64 `json:"debtRatio"`
	DebtToEquity             *float64 `json:"debtToEquity"`
	DebtToAssets             *float64 `json:"debtToAssets"`
	FinancialLeverage        *float64 `json:"financialLeverage"`
	InterestCoverage         *float64 `json:"interestCoverage"`
	CashFlowToDebt           *float64 `json:"cashFlowToDebt"`
	// Valuation Ratios
	PriceEarningsRatio       *float64 `json:"priceEarningsRatio"`
	PriceToBookRatio         *float64 `json:"priceToBookRatio"`
	PriceToSalesRatio        *float64 `json:"priceToSalesRatio"`
	PriceToFreeCashFlows     *float64 `json:"priceToFreeCashFlows"`
	PriceToOperatingCF       *float64 `json:"priceToOperatingCF"`
	EVToRevenue              *float64 `json:"EVToRevenue"`
	EVToEBITDA              *float64 `json:"EVToEBITDA"`
	// Cash Flow Ratios
	CashPerShare              *float64 `json:"cashPerShare"`
	OperatingCashFlowPerShare *float64 `json:"operatingCashFlowPerShare"`
	FreeCashFlowPerShare      *float64 `json:"freeCashFlowPerShare"`
	CashFlowCoverageOfDebt    *float64 `json:"cashFlowCoverageOfDebt"`
	CashFlowToDebtRatio       *float64 `json:"cashFlowToDebtRatio"`
	// Dividend Metrics
	DividendYield *float64 `json:"dividendYield"`
	PayoutRatio  *float64 `json:"payoutRatio"`
}

// KeyMetricsResponse represents the API response for the key metrics endpoint.
type KeyMetricsResponse struct {
	Symbol                  string  `json:"symbol"`
	Date                    string  `json:"date"`
	Period                  string  `json:"period"`
	// Valuation Metrics
	MarketCap               *float64 `json:"marketCap"`
	PERatio                *float64 `json:"peRatio"`
	PEGRatio               *float64 `json:"pegRatio"`
	PriceToBookRatio       *float64 `json:"priceToBookRatio"`
	PriceToSalesRatio      *float64 `json:"priceToSalesRatio"`
	PriceToFreeCashFlows   *float64 `json:"priceToFreeCashFlows"`
	// Profitability Metrics
	NetProfitMargin         *float64 `json:"netProfitMargin"`
	GrossProfitMargin       *float64 `json:"grossProfitMargin"`
	OperatingProfitMargin   *float64 `json:"operatingProfitMargin"`
	ReturnOnEquity          *float64 `json:"returnOnEquity"`
	ReturnOnAssets          *float64 `json:"returnOnAssets"`
	ROIC                    *float64 `json:"roic"`
	// Financial Health
	DebtToEquity             *float64 `json:"debtToEquity"`
	DebtToAssets             *float64 `json:"debtToAssets"`
	CurrentRatio             *float64 `json:"currentRatio"`
	QuickRatio               *float64 `json:"quickRatio"`
	// Per Share Metrics
	RevenuePerShare         *float64 `json:"revenuePerShare"`
	NetIncomePerShare       *float64 `json:"netIncomePerShare"`
	OperatingCFPerShare     *float64 `json:"operatingCashFlowPerShare"`
	FreeCFPerShare          *float64 `json:"freeCashFlowPerShare"`
	BookValuePerShare       *float64 `json:"bookValuePerShare"`
	TangibleAssetPerShare   *float64 `json:"tangibleAssetPerShare"`
	// Growth Metrics
	RevenueGrowth           *float64 `json:"revenueGrowth"`
	NetIncomeGrowth         *float64 `json:"netIncomeGrowth"`
	EPSGrowth               *float64 `json:"epsgrowth"`
	// Other Metrics
	FreeCashFlow            *float64 `json:"freeCashFlow"`
	OperatingCashFlow       *float64 `json:"operatingCashFlow"`
	EBITDA                  *float64 `json:"ebitda"`
	Revenue                 *float64 `json:"revenue"`
	NetIncome               *float64 `json:"netIncome"`
	GrossProfit            *float64 `json:"grossProfit"`
}

// Period constants for API requests.
const (
	PeriodAnnual   = "annual"
	PeriodQuarter  = "quarter"
	PeriodTTM      = "ttm"
)
