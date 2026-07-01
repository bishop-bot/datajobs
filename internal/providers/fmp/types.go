package fmp

// FinancialRatiosResponse represents the API response for the financial ratios endpoint.
type FinancialRatiosResponse struct {
	Symbol string `json:"symbol"`
	Date   string `json:"date"`
	// Profitability Ratios (TTM suffix for TTM endpoint)
	GrossProfitMargin        *float64 `json:"grossProfitMarginTTM"`
	OperatingProfitMargin    *float64 `json:"operatingProfitMarginTTM"`
	NetProfitMargin          *float64 `json:"netProfitMarginTTM"`
	ReturnOnAssets           *float64 `json:"returnOnAssetsTTM"`
	ReturnOnEquity           *float64 `json:"returnOnEquityTTM"`
	ReturnOnCapitalEmployed  *float64 `json:"returnOnCapitalEmployedTTM"`
	ROIC                     *float64 `json:"returnOnInvestedCapitalTTM"`
	// Liquidity Ratios
	CurrentRatio   *float64 `json:"currentRatioTTM"`
	QuickRatio     *float64 `json:"quickRatioTTM"`
	CashRatio      *float64 `json:"cashRatioTTM"`
	WorkingCapital *float64 `json:"workingCapitalTTM"`
	// Efficiency Ratios
	AssetTurnover            *float64 `json:"assetTurnoverTTM"`
	InventoryTurnoverRatio   *float64 `json:"inventoryTurnoverTTM"`
	ReceivablesTurnover      *float64 `json:"receivablesTurnoverTTM"`
	PayablesTurnover         *float64 `json:"payablesTurnoverTTM"`
	DaysSalesOutstanding     *float64 `json:"daysOfSalesOutstandingTTM"`
	DaysInventoryOutstanding *float64 `json:"daysOfInventoryOutstandingTTM"`
	OperatingCycle           *float64 `json:"operatingCycleTTM"`
	DaysPayablesOutstanding  *float64 `json:"daysOfPayablesOutstandingTTM"`
	CashConversionCycle      *float64 `json:"cashConversionCycleTTM"`
	// Leverage Ratios
	DebtRatio         *float64 `json:"debtToAssetsRatioTTM"`
	DebtToEquity      *float64 `json:"debtToEquityRatioTTM"`
	DebtToAssets      *float64 `json:"debtToAssetsRatioTTM"`
	FinancialLeverage *float64 `json:"financialLeverageRatioTTM"`
	InterestCoverage  *float64 `json:"interestCoverageRatioTTM"`
	CashFlowToDebt    *float64 `json:"debtServiceCoverageRatioTTM"`
	// Valuation Ratios
	PriceEarningsRatio   *float64 `json:"priceToEarningsRatioTTM"`
	PriceToBookRatio     *float64 `json:"priceToBookRatioTTM"`
	PriceToSalesRatio    *float64 `json:"priceToSalesRatioTTM"`
	PriceToFreeCashFlows *float64 `json:"priceToFreeCashFlowRatioTTM"`
	PriceToOperatingCF   *float64 `json:"priceToOperatingCashFlowRatioTTM"`
	EVToRevenue          *float64 `json:"evToSalesTTM"`
	EVToEBITDA           *float64 `json:"evToEBITDATTM"`
	// Cash Flow Ratios
	CashPerShare              *float64 `json:"cashPerShareTTM"`
	OperatingCashFlowPerShare *float64 `json:"operatingCashFlowPerShareTTM"`
	FreeCashFlowPerShare      *float64 `json:"freeCashFlowPerShareTTM"`
	CashFlowCoverageOfDebt    *float64 `json:"cashFlowCoverageRatioTTM"`
	CashFlowToDebtRatio       *float64 `json:"operatingCashFlowCoverageRatioTTM"`
	// Dividend Metrics
	DividendYield *float64 `json:"dividendYieldTTM"`
	PayoutRatio   *float64 `json:"dividendPayoutRatioTTM"`
}

// KeyMetricsResponse represents the API response for the key metrics endpoint.
type KeyMetricsResponse struct {
	Symbol string `json:"symbol"`
	Date   string `json:"date"`
	Period string `json:"period"`
	// Valuation Metrics
	MarketCap            *float64 `json:"marketCap"`
	PERatio              *float64 `json:"priceToEarningsRatioTTM"`
	PEGRatio             *float64 `json:"pegRatio"`
	PriceToBookRatio     *float64 `json:"priceToBookRatioTTM"`
	PriceToSalesRatio    *float64 `json:"priceToSalesRatioTTM"`
	PriceToFreeCashFlows *float64 `json:"priceToFreeCashFlowRatioTTM"`
	EnterpriseValue      *float64 `json:"enterpriseValueTTM"`
	// Profitability Metrics
	NetProfitMargin       *float64 `json:"netProfitMarginTTM"`
	GrossProfitMargin     *float64 `json:"grossProfitMarginTTM"`
	OperatingProfitMargin *float64 `json:"operatingProfitMarginTTM"`
	ReturnOnEquity        *float64 `json:"returnOnEquityTTM"`
	ReturnOnAssets        *float64 `json:"returnOnAssetsTTM"`
	ROIC                  *float64 `json:"returnOnInvestedCapitalTTM"`
	// Financial Health
	DebtToEquity *float64 `json:"debtToEquityRatioTTM"`
	DebtToAssets *float64 `json:"debtToAssetsRatioTTM"`
	CurrentRatio *float64 `json:"currentRatioTTM"`
	QuickRatio   *float64 `json:"quickRatioTTM"`
	// Per Share Metrics
	RevenuePerShare       *float64 `json:"revenuePerShareTTM"`
	NetIncomePerShare     *float64 `json:"netIncomePerShareTTM"`
	OperatingCFPerShare   *float64 `json:"operatingCashFlowPerShareTTM"`
	FreeCFPerShare        *float64 `json:"freeCashFlowPerShareTTM"`
	BookValuePerShare     *float64 `json:"bookValuePerShareTTM"`
	TangibleAssetPerShare *float64 `json:"tangibleBookValuePerShareTTM"`
	// Growth Metrics
	RevenueGrowth   *float64 `json:"revenueGrowthTTM"`
	NetIncomeGrowth *float64 `json:"netIncomeGrowthTTM"`
	EPSGrowth       *float64 `json:"earningsYieldTTM"`
	// Other Metrics
	FreeCashFlow      *float64 `json:"freeCashFlowToFirmTTM"`
	OperatingCashFlow *float64 `json:"operatingCashFlowTTM"`
	EBITDA            *float64 `json:"ebitdaMarginTTM"`
	Revenue           *float64 `json:"revenueTTM"`
	NetIncome         *float64 `json:"netIncomeTTM"`
	GrossProfit       *float64 `json:"grossProfitTTM"`
}

// Period constants for API requests.
const (
	PeriodAnnual  = "annual"
	PeriodQuarter = "quarter"
	PeriodTTM     = "ttm"
)
