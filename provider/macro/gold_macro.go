package macro

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// Types
// ============================================================================

// CommodityQuote represents an instrument with full intraday OHLC data.
// Used for instruments available through Sina hf_ API and enhanced Sina Bond API.
type CommodityQuote struct {
	Name       string  `json:"name"`
	Current    float64 `json:"current"`
	Open       float64 `json:"open"`
	High       float64 `json:"high"`
	Low        float64 `json:"low"`
	PrevClose  float64 `json:"prev_close"`
	Settlement float64 `json:"settlement"`
	Bid        float64 `json:"bid"`
	Ask        float64 `json:"ask"`
	ChangePct  float64 `json:"change_pct"`
	DayRange   float64 `json:"day_range"`
	Suffix     string  `json:"suffix,omitempty"`
	Available  bool    `json:"available"`
	UpdateTime string  `json:"update_time,omitempty"`
}

// MacroIndicator represents a simple macro indicator (price + change only).
// Used for sources that don't provide OHLC data (ifnews: DXY, USDJPY).
type MacroIndicator struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	ChangePct float64 `json:"change_pct"`
	Available bool    `json:"available"`
}

// MacroData holds all macro indicators for trading decisions.
type MacroData struct {
	// Commodities (Sina hf_, full intraday OHLC)
	Gold     *CommodityQuote `json:"gold,omitempty"`
	CrudeOil *CommodityQuote `json:"crude_oil,omitempty"`
	Silver   *CommodityQuote `json:"silver,omitempty"`
	Copper   *CommodityQuote `json:"copper,omitempty"`

	// Financial futures (Sina hf_)
	CMEBTC *CommodityQuote `json:"cme_btc,omitempty"`
	SP500  *CommodityQuote `json:"sp500,omitempty"`
	NASDAQ *CommodityQuote `json:"nasdaq,omitempty"`

	// Risk gauges
	VIX        *CommodityQuote `json:"vix,omitempty"`
	US10YYield *CommodityQuote `json:"us_10y_yield,omitempty"`

	// Currency indices (ifnews, price + changePct only)
	DXY    MacroIndicator `json:"dxy"`
	USDJPY MacroIndicator `json:"usd_jpy"`

	// COMEX supplemental (volume/OI)
	ComexVolume string `json:"comex_volume,omitempty"`
	ComexOI     string `json:"comex_oi,omitempty"`

	FetchedAt time.Time `json:"fetched_at"`
	Errors    []string  `json:"errors,omitempty"`
}

// GoldMacroData is a deprecated alias for MacroData (backward compatibility).
type GoldMacroData = MacroData

// ============================================================================
// Cache & Fetch
// ============================================================================

var (
	cachedMacro *MacroData
	cachedAt    time.Time
	cacheMu     sync.RWMutex
	cacheTTL    = 5 * time.Minute
	httpClient  = &http.Client{Timeout: 15 * time.Second}
)

// FetchMacroData returns cached macro data or fetches fresh data if cache expired.
func FetchMacroData() *MacroData {
	cacheMu.RLock()
	if cachedMacro != nil && time.Since(cachedAt) < cacheTTL {
		cacheMu.RUnlock()
		return cachedMacro
	}
	cacheMu.RUnlock()

	cacheMu.Lock()
	defer cacheMu.Unlock()

	if cachedMacro != nil && time.Since(cachedAt) < cacheTTL {
		return cachedMacro
	}

	data := &MacroData{FetchedAt: time.Now()}

	var wg sync.WaitGroup
	var mu sync.Mutex
	addError := func(msg string) {
		mu.Lock()
		data.Errors = append(data.Errors, msg)
		mu.Unlock()
	}

	wg.Add(4)
	go func() { defer wg.Done(); fetchSinaFutures(data, addError) }()
	go func() { defer wg.Done(); fetchFromIfnews(data, addError) }()
	go func() { defer wg.Done(); fetchUS10YFromSina(data, addError) }()
	go func() { defer wg.Done(); fetchCOMEXFromCME(data, addError) }()
	wg.Wait()

	available := 0
	total := 0
	for _, q := range []*CommodityQuote{data.Gold, data.CrudeOil, data.Silver, data.Copper,
		data.CMEBTC, data.SP500, data.NASDAQ, data.VIX, data.US10YYield} {
		total++
		if q != nil && q.Available {
			available++
		}
	}
	for _, ind := range []MacroIndicator{data.DXY, data.USDJPY} {
		total++
		if ind.Available {
			available++
		}
	}

	if available < total {
		logger.Infof("⚠️ [Macro] %d/%d indicators available (missing %d)", available, total, total-available)
		if len(data.Errors) > 0 {
			for _, e := range data.Errors {
				logger.Infof("  ↳ %s", e)
			}
		}
	} else {
		logger.Infof("✅ [Macro] All %d/%d macro indicators fetched successfully", available, total)
	}

	cachedMacro = data
	cachedAt = time.Now()
	return data
}

// FetchGoldMacro is a deprecated wrapper for FetchMacroData (backward compatibility).
func FetchGoldMacro() *GoldMacroData {
	return FetchMacroData()
}

// ============================================================================
// Sina hf_ Futures Batch API (8 instruments, 1 HTTP request)
// ============================================================================

type sinaInstrument struct {
	code   string
	name   string
	suffix string
	target **CommodityQuote
}

func fetchSinaFutures(data *MacroData, addError func(string)) {
	instruments := []sinaInstrument{
		{"hf_XAU", "黄金 (XAU)", "", &data.Gold},
		{"hf_CL", "原油 (CL)", "", &data.CrudeOil},
		{"hf_SI", "白银 (SI)", "", &data.Silver},
		{"hf_HG", "铜 (HG)", "", &data.Copper},
		{"hf_BTC", "CME BTC", "", &data.CMEBTC},
		{"hf_ES", "标普500 (ES)", "", &data.SP500},
		{"hf_NQ", "纳斯达克 (NQ)", "", &data.NASDAQ},
		{"hf_VX", "VIX", "", &data.VIX},
	}

	codes := make([]string, len(instruments))
	for i, inst := range instruments {
		codes[i] = inst.code
	}

	url := "http://w.sinajs.cn/?list=" + strings.Join(codes, ",")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		addError(fmt.Sprintf("sina futures: %v", err))
		return
	}
	req.Header.Set("Host", "w.sinajs.cn")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Referer", "https://gu.sina.cn/ft/hq/hf.php?symbol=XAU")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Sec-Fetch-Mode", "no-cors")
	req.Header.Set("Sec-Fetch-Dest", "script")

	resp, err := httpClient.Do(req)
	if err != nil {
		addError(fmt.Sprintf("sina futures: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		addError(fmt.Sprintf("sina futures: HTTP %d", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		addError(fmt.Sprintf("sina futures read: %v", err))
		return
	}

	parsed := parseSinaResponse(string(body))

	var failed []string
	for _, inst := range instruments {
		fields, ok := parsed[inst.code]
		if !ok || len(fields) < 9 {
			failed = append(failed, inst.name)
			continue
		}
		q := parseSinaHFFields(fields, inst.name, inst.suffix)
		if q != nil {
			*inst.target = q
		} else {
			failed = append(failed, inst.name)
		}
	}
	if len(failed) > 0 {
		addError(fmt.Sprintf("sina futures: %d/%d unavailable (%s)", len(failed), len(instruments), strings.Join(failed, ", ")))
	}
}

// parseSinaResponse parses Sina's response into a map of code → fields.
// Each line: var hq_str_hf_XAU="f0,f1,...";
func parseSinaResponse(body string) map[string][]string {
	result := make(map[string][]string)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue
		}
		code := strings.TrimPrefix(line[:eqIdx], "var hq_str_")

		q1 := strings.Index(line, "\"")
		q2 := strings.LastIndex(line, "\"")
		if q1 < 0 || q2 <= q1 {
			continue
		}
		dataStr := line[q1+1 : q2]
		if dataStr == "" {
			continue
		}
		result[code] = strings.Split(dataStr, ",")
	}
	return result
}

// parseSinaHFFields parses comma-separated fields into a CommodityQuote.
// Format: 0=Open, 1=Current, 2=Bid, 3=Ask, 4=High, 5=Low, 6=Time, 7=PrevClose, 8=Settlement
func parseSinaHFFields(fields []string, name, suffix string) *CommodityQuote {
	parseF := func(idx int) float64 {
		if idx >= len(fields) {
			return 0
		}
		v, _ := strconv.ParseFloat(strings.TrimSpace(fields[idx]), 64)
		return v
	}

	open := parseF(0)
	current := parseF(1)
	bid := parseF(2)
	ask := parseF(3)
	high := parseF(4)
	low := parseF(5)
	prevClose := parseF(7)
	settlement := parseF(8)

	timeStr := ""
	if len(fields) > 6 {
		timeStr = strings.TrimSpace(fields[6])
	}

	if current == 0 {
		if bid > 0 && ask > 0 {
			current = (bid + ask) / 2
		} else if bid > 0 {
			current = bid
		} else if ask > 0 {
			current = ask
		}
	}
	if current == 0 {
		return nil
	}

	changePct := 0.0
	if prevClose > 0 {
		changePct = (current - prevClose) / prevClose * 100
	}

	dayRange := 0.0
	if prevClose > 0 && high > 0 && low > 0 {
		dayRange = (high - low) / prevClose * 100
	}

	return &CommodityQuote{
		Name:       name,
		Current:    current,
		Open:       open,
		High:       high,
		Low:        low,
		PrevClose:  prevClose,
		Settlement: settlement,
		Bid:        bid,
		Ask:        ask,
		ChangePct:  changePct,
		DayRange:   dayRange,
		Suffix:     suffix,
		Available:  true,
		UpdateTime: timeStr,
	}
}

// ============================================================================
// ifnews API (DXY, USDJPY only)
// ============================================================================

func fetchFromIfnews(data *MacroData, addError func(string)) {
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
		Name       string  `json:"name"`
		Price      string  `json:"price"`
		PriceLimit float64 `json:"priceLimit"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		addError(fmt.Sprintf("ifnews parse: %v", err))
		return
	}

	nameMap := map[string]*MacroIndicator{
		"美元指数":   &data.DXY,
		"日元/1美元": &data.USDJPY,
	}
	displayNames := map[string]string{
		"美元指数":   "DXY (美元指数)",
		"日元/1美元": "USD/JPY",
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

// ============================================================================
// Sina US 10Y Bond Yield API (enhanced with OHLC from minute data)
// ============================================================================

func extractJSON(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return raw[start : end+1]
	}
	return raw
}

func fetchUS10YFromSina(data *MacroData, addError func(string)) {
	url := "https://bond.finance.sina.com.cn/hq/gb/min?symbol=us10yt"
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

	jsonStr := extractJSON(string(body))

	var result struct {
		Code   int `json:"code"`
		Result struct {
			Data [][]interface{} `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		addError(fmt.Sprintf("sina us10y parse: %v", err))
		return
	}

	entries := result.Result.Data
	if len(entries) == 0 {
		addError("sina us10y: no data")
		return
	}

	// First entry: [time, bond_price, _, yield, date, prev_close_yield]
	first := entries[0]
	var prevClose float64
	if len(first) >= 6 {
		prevStr, _ := first[5].(string)
		prevClose, _ = strconv.ParseFloat(prevStr, 64)
	}

	// Open yield from first entry index 3
	var openYield float64
	if len(first) >= 4 {
		openStr, _ := first[3].(string)
		openYield, _ = strconv.ParseFloat(openStr, 64)
	}
	if openYield == 0 && len(entries) > 1 && len(entries[1]) >= 2 {
		s, _ := entries[1][1].(string)
		openYield, _ = strconv.ParseFloat(s, 64)
	}

	// Scan all entries for current, high, low
	var current, high, low float64
	initialized := false

	for i, entry := range entries {
		if len(entry) < 2 {
			continue
		}
		var yield float64
		if i == 0 && len(entry) >= 4 {
			yStr, _ := entry[3].(string)
			yield, _ = strconv.ParseFloat(yStr, 64)
		} else {
			yStr, _ := entry[1].(string)
			yield, _ = strconv.ParseFloat(yStr, 64)
		}
		if yield > 0 {
			current = yield
			if !initialized {
				high = yield
				low = yield
				initialized = true
			} else {
				if yield > high {
					high = yield
				}
				if yield < low {
					low = yield
				}
			}
		}
	}

	if current == 0 {
		addError("sina us10y: no valid yield data")
		return
	}

	changePct := 0.0
	if prevClose > 0 {
		changePct = (current - prevClose) / prevClose * 100
	}
	dayRange := 0.0
	if prevClose > 0 && high > 0 && low > 0 {
		dayRange = (high - low) / prevClose * 100
	}

	data.US10YYield = &CommodityQuote{
		Name:      "US 10Y (美国10年期国债)",
		Current:   current,
		Open:      openYield,
		High:      high,
		Low:       low,
		PrevClose: prevClose,
		ChangePct: changePct,
		DayRange:  dayRange,
		Suffix:    "%",
		Available: true,
	}
}

// ============================================================================
// CME COMEX (Volume/OI supplemental data only)
// ============================================================================

func fetchCOMEXFromCME(data *MacroData, addError func(string)) {
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
			Volume       string `json:"volume"`
			IsFrontMonth bool   `json:"isFrontMonth"`
		} `json:"quotes"`
	}
	if err := json.Unmarshal(body, &cmeData); err != nil {
		addError(fmt.Sprintf("cme parse: %v", err))
		return
	}

	for _, q := range cmeData.Quotes {
		if q.IsFrontMonth {
			vol := strings.ReplaceAll(q.Volume, ",", "")
			if vol != "" && vol != "-" {
				data.ComexVolume = vol
			}
			break
		}
	}

	fetchCOMEXVolume(data, addError)
}

func fetchCOMEXVolume(data *MacroData, addError func(string)) {
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

// ============================================================================
// Prompt Formatting
// ============================================================================

// FormatForPromptZh formats macro data as a Chinese prompt section.
func (d *MacroData) FormatForPromptZh() string {
	if d == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 宏观面数据\n\n")

	writeQuoteSectionZh(&sb, "### 大宗商品\n", []quoteWithNote{
		{d.Gold, "避险核心"},
		{d.CrudeOil, "地缘/通胀代理"},
		{d.Silver, "贵金属联动"},
		{d.Copper, "经济晴雨表"},
	})

	writeQuoteSectionZh(&sb, "### 金融期货\n", []quoteWithNote{
		{d.CMEBTC, "BTC期现溢价参考"},
		{d.SP500, "风险偏好"},
		{d.NASDAQ, "科技风险偏好"},
	})

	writeQuoteSectionZh(&sb, "### 风险指标\n", []quoteWithNote{
		{d.VIX, ">25利多黄金/BTC避险"},
		{d.US10YYield, "收益率↑=持金成本↑=金价承压"},
	})

	hasIndicator := false
	for _, item := range []struct {
		ind  MacroIndicator
		note string
	}{
		{d.DXY, "黄金/BTC反向指标"},
		{d.USDJPY, "避险联动(↓=避险买金)"},
	} {
		if item.ind.Available {
			if !hasIndicator {
				sb.WriteString("### 汇率指数\n")
				hasIndicator = true
			}
			arrow := "↑"
			if item.ind.ChangePct < 0 {
				arrow = "↓"
			}
			sb.WriteString(fmt.Sprintf("- %s: %.2f (%s%.2f%%) — %s\n",
				item.ind.Name, item.ind.Value, arrow, abs(item.ind.ChangePct), item.note))
		}
	}

	if d.ComexVolume != "" || d.ComexOI != "" {
		parts := []string{}
		if d.ComexVolume != "" {
			parts = append(parts, fmt.Sprintf("成交量: %s", d.ComexVolume))
		}
		if d.ComexOI != "" {
			parts = append(parts, fmt.Sprintf("持仓量: %s", d.ComexOI))
		}
		sb.WriteString(fmt.Sprintf("\n(COMEX黄金: %s)\n", strings.Join(parts, ", ")))
	}

	if len(d.Errors) > 0 {
		sb.WriteString(fmt.Sprintf("(注意: %d个数据源暂不可用)\n", len(d.Errors)))
	}
	sb.WriteString("\n")

	return sb.String()
}

// FormatForPromptEn formats macro data as an English prompt section.
func (d *MacroData) FormatForPromptEn() string {
	if d == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Macro Data\n\n")

	writeQuoteSectionEn(&sb, "### Commodities\n", []quoteWithNote{
		{d.Gold, "safe haven core"},
		{d.CrudeOil, "geopolitical/inflation proxy"},
		{d.Silver, "precious metals correlation"},
		{d.Copper, "economic health indicator"},
	})

	writeQuoteSectionEn(&sb, "### Financial Futures\n", []quoteWithNote{
		{d.CMEBTC, "BTC futures premium/discount"},
		{d.SP500, "risk appetite"},
		{d.NASDAQ, "tech risk appetite"},
	})

	writeQuoteSectionEn(&sb, "### Risk Gauges\n", []quoteWithNote{
		{d.VIX, ">25 bullish gold/BTC (fear)"},
		{d.US10YYield, "yield up = gold cost up"},
	})

	hasIndicator := false
	for _, item := range []struct {
		ind  MacroIndicator
		note string
	}{
		{d.DXY, "inverse correlator for gold/BTC"},
		{d.USDJPY, "safe-haven pair (down = risk-off)"},
	} {
		if item.ind.Available {
			if !hasIndicator {
				sb.WriteString("### Currency Indices\n")
				hasIndicator = true
			}
			arrow := "↑"
			if item.ind.ChangePct < 0 {
				arrow = "↓"
			}
			sb.WriteString(fmt.Sprintf("- %s: %.2f (%s%.2f%%) — %s\n",
				item.ind.Name, item.ind.Value, arrow, abs(item.ind.ChangePct), item.note))
		}
	}

	if d.ComexVolume != "" || d.ComexOI != "" {
		parts := []string{}
		if d.ComexVolume != "" {
			parts = append(parts, fmt.Sprintf("Volume: %s", d.ComexVolume))
		}
		if d.ComexOI != "" {
			parts = append(parts, fmt.Sprintf("OI: %s", d.ComexOI))
		}
		sb.WriteString(fmt.Sprintf("\n(COMEX Gold: %s)\n", strings.Join(parts, ", ")))
	}

	if len(d.Errors) > 0 {
		sb.WriteString(fmt.Sprintf("(Note: %d data sources unavailable)\n", len(d.Errors)))
	}
	sb.WriteString("\n")

	return sb.String()
}

// ============================================================================
// Format Helpers
// ============================================================================

type quoteWithNote struct {
	q    *CommodityQuote
	note string
}

func writeQuoteSectionZh(sb *strings.Builder, header string, items []quoteWithNote) {
	hasAny := false
	for _, item := range items {
		if item.q != nil && item.q.Available {
			if !hasAny {
				sb.WriteString(header)
				hasAny = true
			}
			sb.WriteString(formatQuoteLineZh(item.q, item.note))
		}
	}
}

func writeQuoteSectionEn(sb *strings.Builder, header string, items []quoteWithNote) {
	hasAny := false
	for _, item := range items {
		if item.q != nil && item.q.Available {
			if !hasAny {
				sb.WriteString(header)
				hasAny = true
			}
			sb.WriteString(formatQuoteLineEn(item.q, item.note))
		}
	}
}

func formatQuoteLineZh(q *CommodityQuote, note string) string {
	pf := autoPriceFmt(q.Current)
	s := q.Suffix
	arrow := "↑"
	if q.ChangePct < 0 {
		arrow = "↓"
	}

	parts := []string{
		fmt.Sprintf("- %s: %s%s (%s%.2f%%)", q.Name, fmt.Sprintf(pf, q.Current), s, arrow, abs(q.ChangePct)),
	}

	if q.Open > 0 {
		openPct := 0.0
		if q.Open > 0 {
			openPct = (q.Current - q.Open) / q.Open * 100
		}
		parts = append(parts, fmt.Sprintf("今开 %s%s (%+.2f%%)", fmt.Sprintf(pf, q.Open), s, openPct))
	}

	if q.High > 0 && q.Low > 0 {
		parts = append(parts, fmt.Sprintf("区间 %s~%s%s (%.2f%%)",
			fmt.Sprintf(pf, q.Low), fmt.Sprintf(pf, q.High), s, q.DayRange))
	}

	line := strings.Join(parts, " | ")
	if note != "" {
		line += " — " + note
	}
	return line + "\n"
}

func formatQuoteLineEn(q *CommodityQuote, note string) string {
	pf := autoPriceFmt(q.Current)
	s := q.Suffix
	arrow := "↑"
	if q.ChangePct < 0 {
		arrow = "↓"
	}

	parts := []string{
		fmt.Sprintf("- %s: %s%s (%s%.2f%%)", q.Name, fmt.Sprintf(pf, q.Current), s, arrow, abs(q.ChangePct)),
	}

	if q.Open > 0 {
		openPct := 0.0
		if q.Open > 0 {
			openPct = (q.Current - q.Open) / q.Open * 100
		}
		parts = append(parts, fmt.Sprintf("Open %s%s (%+.2f%%)", fmt.Sprintf(pf, q.Open), s, openPct))
	}

	if q.High > 0 && q.Low > 0 {
		parts = append(parts, fmt.Sprintf("Range %s~%s%s (%.2f%%)",
			fmt.Sprintf(pf, q.Low), fmt.Sprintf(pf, q.High), s, q.DayRange))
	}

	line := strings.Join(parts, " | ")
	if note != "" {
		line += " — " + note
	}
	return line + "\n"
}

func autoPriceFmt(v float64) string {
	absV := abs(v)
	switch {
	case absV >= 10000:
		return "%.0f"
	case absV >= 100:
		return "%.2f"
	case absV >= 10:
		return "%.2f"
	default:
		return "%.3f"
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
