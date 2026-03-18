package macro

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MacroIndicator represents a single macro indicator value.
type MacroIndicator struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	ChangePct float64 `json:"change_pct"`
	Available bool    `json:"available"`
}

// GoldMacroData holds all macro indicators relevant to gold trading.
type GoldMacroData struct {
	DXY        MacroIndicator `json:"dxy"`
	US10YYield MacroIndicator `json:"us_10y_yield"`
	VIX        MacroIndicator `json:"vix"`
	USDJPY     MacroIndicator `json:"usd_jpy"`
	SP500      MacroIndicator `json:"sp500"`
	NASDAQ     MacroIndicator `json:"nasdaq"`

	ComexPrice    float64 `json:"comex_price,omitempty"`
	ComexChangePct float64 `json:"comex_change_pct,omitempty"`
	ComexVolume   string  `json:"comex_volume,omitempty"`
	ComexOI       string  `json:"comex_oi,omitempty"`
	ComexAvailable bool   `json:"comex_available"`

	FetchedAt time.Time `json:"fetched_at"`
	Errors    []string  `json:"errors,omitempty"`
}

var (
	cachedData  *GoldMacroData
	cachedAt    time.Time
	cacheMu     sync.RWMutex
	cacheTTL    = 5 * time.Minute
	httpClient  = &http.Client{Timeout: 10 * time.Second}
)

// FetchGoldMacro returns cached macro data or fetches fresh data if cache expired.
func FetchGoldMacro() *GoldMacroData {
	cacheMu.RLock()
	if cachedData != nil && time.Since(cachedAt) < cacheTTL {
		cacheMu.RUnlock()
		return cachedData
	}
	cacheMu.RUnlock()

	cacheMu.Lock()
	defer cacheMu.Unlock()

	// Double-check after acquiring write lock
	if cachedData != nil && time.Since(cachedAt) < cacheTTL {
		return cachedData
	}

	data := &GoldMacroData{FetchedAt: time.Now()}

	var wg sync.WaitGroup
	var mu sync.Mutex // protects data.Errors

	addError := func(msg string) {
		mu.Lock()
		data.Errors = append(data.Errors, msg)
		mu.Unlock()
	}

	wg.Add(4)
	go func() { defer wg.Done(); fetchFromIfnews(data, addError) }()
	go func() { defer wg.Done(); fetchVIXFromSina(data, addError) }()
	go func() { defer wg.Done(); fetchUS10YFromSina(data, addError) }()
	go func() { defer wg.Done(); fetchCOMEXFromCME(data, addError) }()
	wg.Wait()

	available := 0
	total := 6
	for _, ind := range []MacroIndicator{data.DXY, data.US10YYield, data.VIX, data.USDJPY, data.SP500, data.NASDAQ} {
		if ind.Available {
			available++
		}
	}
	if data.ComexAvailable {
		available++
		total++
	}

	if len(data.Errors) > 0 {
		logger.Infof("⚠️ [Macro] %d/%d indicators available, errors: %v", available, total, data.Errors)
	} else {
		logger.Infof("✅ [Macro] All %d/%d gold macro indicators fetched successfully", available, total)
	}

	cachedData = data
	cachedAt = time.Now()
	return data
}

// ── ifnews API ──────────────────────────────────────────────────────────────

func fetchFromIfnews(data *GoldMacroData, addError func(string)) {
	resp, err := httpClient.Get("http://worldmap.ifnews.com/chinamap/china/financialData?type=all")
	if err != nil {
		addError(fmt.Sprintf("ifnews: %v", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		addError(fmt.Sprintf("ifnews read: %v", err))
		return
	}

	var items []struct {
		Continent   string  `json:"continent"`
		Name        string  `json:"name"`
		Price       string  `json:"price"`
		PriceLimit  float64 `json:"priceLimit"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		addError(fmt.Sprintf("ifnews parse: %v", err))
		return
	}

	nameMap := map[string]*MacroIndicator{
		"美元指数":    &data.DXY,
		"日元/1美元":  &data.USDJPY,
		"标普500指数":  &data.SP500,
		"纳斯达克指数": &data.NASDAQ,
	}
	displayNames := map[string]string{
		"美元指数":    "DXY (美元指数)",
		"日元/1美元":  "USD/JPY",
		"标普500指数":  "S&P 500",
		"纳斯达克指数": "NASDAQ",
	}

	for _, item := range items {
		trimmed := strings.TrimSpace(item.Name)
		if ind, ok := nameMap[trimmed]; ok {
			price, _ := strconv.ParseFloat(strings.TrimSpace(item.Price), 64)
			if price > 0 {
				ind.Name = displayNames[trimmed]
				ind.Value = price
				ind.ChangePct = item.PriceLimit
				ind.Available = true
			}
		}
	}
}

// ── Sina VIX API ────────────────────────────────────────────────────────────

func fetchVIXFromSina(data *GoldMacroData, addError func(string)) {
	resp, err := httpClient.Get("https://gi.finance.sina.com.cn/hq/min?symbol=VIX")
	if err != nil {
		addError(fmt.Sprintf("sina vix: %v", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		addError(fmt.Sprintf("sina vix read: %v", err))
		return
	}

	var result struct {
		Code   int    `json:"code"`
		Result struct {
			Data [][]interface{} `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		addError(fmt.Sprintf("sina vix parse: %v", err))
		return
	}

	entries := result.Result.Data
	if len(entries) == 0 {
		addError("sina vix: no data")
		return
	}

	// Latest VIX value from last entry
	last := entries[len(entries)-1]
	if len(last) < 2 {
		addError("sina vix: invalid entry")
		return
	}
	latestStr, _ := last[1].(string)
	latest, _ := strconv.ParseFloat(latestStr, 64)

	// Previous close from first entry (index 5 if available)
	var prevClose float64
	first := entries[0]
	if len(first) >= 6 {
		prevStr, _ := first[5].(string)
		prevClose, _ = strconv.ParseFloat(prevStr, 64)
	}

	changePct := 0.0
	if prevClose > 0 {
		changePct = (latest - prevClose) / prevClose * 100
	}

	if latest > 0 {
		data.VIX = MacroIndicator{
			Name:      "VIX (恐慌指数)",
			Value:     latest,
			ChangePct: changePct,
			Available: true,
		}
	}
}

// ── Sina US 10Y Bond Yield API ──────────────────────────────────────────────

var jsonpRe = regexp.MustCompile(`\w+\s*=\s*\((\{.+\})\)`)

func fetchUS10YFromSina(data *GoldMacroData, addError func(string)) {
	url := "https://bond.finance.sina.com.cn/hq/gb/min?symbol=us10yt&callback=cb"
	resp, err := httpClient.Get(url)
	if err != nil {
		addError(fmt.Sprintf("sina us10y: %v", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		addError(fmt.Sprintf("sina us10y read: %v", err))
		return
	}

	// Strip JSONP wrapper: cb({"code":0,...})
	raw := string(body)
	matches := jsonpRe.FindStringSubmatch(raw)
	if len(matches) < 2 {
		// Try parsing as plain JSON
		matches = []string{"", raw}
	}

	var result struct {
		Code   int `json:"code"`
		Result struct {
			Data [][]interface{} `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(matches[1]), &result); err != nil {
		addError(fmt.Sprintf("sina us10y parse: %v", err))
		return
	}

	entries := result.Result.Data
	if len(entries) == 0 {
		addError("sina us10y: no data")
		return
	}

	// Latest yield from last entry (index 1)
	last := entries[len(entries)-1]
	if len(last) < 2 {
		addError("sina us10y: invalid entry")
		return
	}
	latestStr, _ := last[1].(string)
	latest, _ := strconv.ParseFloat(latestStr, 64)

	// Previous close yield from first entry (index 5 if present, or index 3 for yield)
	var prevClose float64
	first := entries[0]
	if len(first) >= 6 {
		prevStr, _ := first[5].(string)
		prevClose, _ = strconv.ParseFloat(prevStr, 64)
	}

	changePct := 0.0
	if prevClose > 0 {
		changePct = (latest - prevClose) / prevClose * 100
	}

	if latest > 0 {
		data.US10YYield = MacroIndicator{
			Name:      "US 10Y Yield (美国10年期国债)",
			Value:     latest,
			ChangePct: changePct,
			Available: true,
		}
	}
}

// ── CME COMEX Gold Futures API ──────────────────────────────────────────────

func fetchCOMEXFromCME(data *GoldMacroData, addError func(string)) {
	url := "https://www.cmegroup.com/CmeWS/mvc/quotes/v2/437?isProtected&_t=" + strconv.FormatInt(time.Now().UnixMilli(), 10)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		addError(fmt.Sprintf("cme: %v", err))
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		addError(fmt.Sprintf("cme: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		addError(fmt.Sprintf("cme: HTTP %d", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		addError(fmt.Sprintf("cme read: %v", err))
		return
	}

	var cmeData struct {
		Quotes []struct {
			Last            string `json:"last"`
			PercentageChange string `json:"percentageChange"`
			Volume          string `json:"volume"`
			IsFrontMonth    bool   `json:"isFrontMonth"`
			ExpirationMonth string `json:"expirationMonth"`
		} `json:"quotes"`
	}
	if err := json.Unmarshal(body, &cmeData); err != nil {
		addError(fmt.Sprintf("cme parse: %v", err))
		return
	}

	for _, q := range cmeData.Quotes {
		if q.IsFrontMonth && q.Last != "-" {
			price, _ := strconv.ParseFloat(q.Last, 64)
			changePctStr := strings.TrimSuffix(strings.TrimPrefix(q.PercentageChange, "+"), "%")
			changePct, _ := strconv.ParseFloat(changePctStr, 64)
			vol := strings.ReplaceAll(q.Volume, ",", "")

			if price > 0 {
				data.ComexPrice = price
				data.ComexChangePct = changePct
				data.ComexVolume = vol
				data.ComexAvailable = true
				logger.Debugf("[Macro] COMEX front month %s: $%.1f (%s%%), vol=%s",
					q.ExpirationMonth, price, q.PercentageChange, vol)
			}
			break
		}
	}

	// Try fetching OI from volume endpoint (yesterday's date for settled data)
	fetchCOMEXVolume(data, addError)
}

func fetchCOMEXVolume(data *GoldMacroData, addError func(string)) {
	yesterday := time.Now().AddDate(0, 0, -1).Format("20060102")
	url := fmt.Sprintf("https://www.cmegroup.com/CmeWS/mvc/Volume/Details/F/437/%s/F?tradeDate=%s&pageSize=500&isProtected&_t=%d",
		yesterday, yesterday, time.Now().UnixMilli())

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var volData struct {
		Totals struct {
			TotalVolume string `json:"totalVolume"`
			AtClose     string `json:"atClose"`
		} `json:"totals"`
	}
	if err := json.Unmarshal(body, &volData); err != nil {
		return
	}

	if volData.Totals.AtClose != "" && volData.Totals.AtClose != "-" {
		data.ComexOI = volData.Totals.AtClose
	}
	if volData.Totals.TotalVolume != "" && volData.Totals.TotalVolume != "-" && data.ComexVolume == "" {
		data.ComexVolume = volData.Totals.TotalVolume
	}
}

// ── Prompt Formatting ───────────────────────────────────────────────────────

// FormatForPromptZh formats macro data as a concise Chinese prompt section.
func (d *GoldMacroData) FormatForPromptZh() string {
	if d == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 宏观面数据 (黄金关键驱动因子)\n")

	indicators := []struct {
		ind    MacroIndicator
		note   string
		format string
	}{
		{d.DXY, "黄金第一反向指标", "%.2f"},
		{d.US10YYield, "黄金持有成本(收益率越高金价越承压)", "%.3f%%"},
		{d.VIX, "市场恐慌度(>25利多黄金, <15利空)", "%.2f"},
		{d.USDJPY, "避险资产联动(下跌=避险买金)", "%.2f"},
		{d.SP500, "风险偏好(暴跌=避险买金)", "%.2f"},
		{d.NASDAQ, "科技股风险偏好", "%.2f"},
	}

	for _, item := range indicators {
		if !item.ind.Available {
			continue
		}
		arrow := "↑"
		if item.ind.ChangePct < 0 {
			arrow = "↓"
		}
		valStr := fmt.Sprintf(item.format, item.ind.Value)
		sb.WriteString(fmt.Sprintf("- %s: %s (%s%.2f%%) — %s\n",
			item.ind.Name, valStr, arrow, abs(item.ind.ChangePct), item.note))
	}

	if d.ComexAvailable {
		sb.WriteString(fmt.Sprintf("- COMEX黄金主力合约: $%.1f (%.2f%%)", d.ComexPrice, d.ComexChangePct))
		if d.ComexVolume != "" {
			sb.WriteString(fmt.Sprintf(", 成交量: %s", d.ComexVolume))
		}
		if d.ComexOI != "" {
			sb.WriteString(fmt.Sprintf(", 持仓量: %s", d.ComexOI))
		}
		sb.WriteString("\n")
	}

	if len(d.Errors) > 0 {
		sb.WriteString(fmt.Sprintf("(注意: %d个数据源暂不可用)\n", len(d.Errors)))
	}
	sb.WriteString("\n")

	return sb.String()
}

// FormatForPromptEn formats macro data as a concise English prompt section.
func (d *GoldMacroData) FormatForPromptEn() string {
	if d == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Macro Data (Gold Key Drivers)\n")

	indicators := []struct {
		ind    MacroIndicator
		note   string
		format string
	}{
		{d.DXY, "primary inverse correlator for gold", "%.2f"},
		{d.US10YYield, "opportunity cost of holding gold", "%.3f%%"},
		{d.VIX, "fear gauge (>25 bullish gold, <15 bearish)", "%.2f"},
		{d.USDJPY, "safe-haven pair (decline = risk-off = gold bullish)", "%.2f"},
		{d.SP500, "risk appetite (crash = flight to gold)", "%.2f"},
		{d.NASDAQ, "tech risk appetite", "%.2f"},
	}

	for _, item := range indicators {
		if !item.ind.Available {
			continue
		}
		arrow := "↑"
		if item.ind.ChangePct < 0 {
			arrow = "↓"
		}
		valStr := fmt.Sprintf(item.format, item.ind.Value)
		sb.WriteString(fmt.Sprintf("- %s: %s (%s%.2f%%) — %s\n",
			item.ind.Name, valStr, arrow, abs(item.ind.ChangePct), item.note))
	}

	if d.ComexAvailable {
		sb.WriteString(fmt.Sprintf("- COMEX Gold Front Month: $%.1f (%.2f%%)", d.ComexPrice, d.ComexChangePct))
		if d.ComexVolume != "" {
			sb.WriteString(fmt.Sprintf(", Volume: %s", d.ComexVolume))
		}
		if d.ComexOI != "" {
			sb.WriteString(fmt.Sprintf(", OI: %s", d.ComexOI))
		}
		sb.WriteString("\n")
	}

	if len(d.Errors) > 0 {
		sb.WriteString(fmt.Sprintf("(Note: %d data sources unavailable)\n", len(d.Errors)))
	}
	sb.WriteString("\n")

	return sb.String()
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
