package kernel

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/provider/macro"
	"nofx/store"
	"strings"
	"time"
)

// ============================================================================
// Grid Trading Context and Types
// ============================================================================

// GridLevelInfo represents a single grid level's current state
type GridLevelInfo struct {
	Index          int     `json:"index"`            // Level index (0 = lowest)
	Price          float64 `json:"price"`            // Target price for this level
	State          string  `json:"state"`            // "empty", "pending", "filled"
	Side           string  `json:"side"`             // "buy" or "sell"
	OrderID        string  `json:"order_id"`         // Current order ID (if pending)
	OrderQuantity  float64 `json:"order_quantity"`   // Order quantity
	PositionSize   float64 `json:"position_size"`    // Position size (if filled)
	PositionEntry  float64 `json:"position_entry"`   // Entry price (if filled)
	AllocatedUSD   float64 `json:"allocated_usd"`    // USD allocated to this level
	UnrealizedPnL  float64 `json:"unrealized_pnl"`   // Unrealized P&L (if filled)
}

// GridContext contains all information needed for AI grid decision making
type GridContext struct {
	// Basic info
	Symbol       string    `json:"symbol"`
	CurrentTime  string    `json:"current_time"`
	CurrentPrice float64   `json:"current_price"`

	// Grid configuration
	GridCount       int     `json:"grid_count"`
	TotalInvestment float64 `json:"total_investment"`
	Leverage        int     `json:"leverage"`
	UpperPrice      float64 `json:"upper_price"`
	LowerPrice      float64 `json:"lower_price"`
	GridSpacing     float64 `json:"grid_spacing"`
	Distribution    string  `json:"distribution"`

	// Grid state
	Levels           []GridLevelInfo `json:"levels"`
	ActiveOrderCount int             `json:"active_order_count"`
	FilledLevelCount int             `json:"filled_level_count"`
	IsPaused         bool            `json:"is_paused"`

	// Market data
	ATR14          float64 `json:"atr14"`
	BollingerUpper float64 `json:"bollinger_upper"`
	BollingerMiddle float64 `json:"bollinger_middle"`
	BollingerLower float64 `json:"bollinger_lower"`
	BollingerWidth float64 `json:"bollinger_width"` // Percentage
	EMA20          float64 `json:"ema20"`
	EMA50          float64 `json:"ema50"`
	EMADistance    float64 `json:"ema_distance"` // Percentage
	RSI14          float64 `json:"rsi14"`
	MACD           float64 `json:"macd"`
	MACDSignal     float64 `json:"macd_signal"`
	MACDHistogram  float64 `json:"macd_histogram"`
	FundingRate    float64 `json:"funding_rate"`
	Volume24h      float64 `json:"volume_24h"`
	PriceChange1h  float64 `json:"price_change_1h"`
	PriceChange4h  float64 `json:"price_change_4h"`

	// Account info
	TotalEquity      float64 `json:"total_equity"`
	AvailableBalance float64 `json:"available_balance"`
	CurrentPosition  float64 `json:"current_position"` // Net position size
	UnrealizedPnL    float64 `json:"unrealized_pnl"`

	// Performance
	TotalProfit   float64 `json:"total_profit"`
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	MaxDrawdown   float64 `json:"max_drawdown"`
	DailyPnL      float64 `json:"daily_pnl"`

	// Box indicators (Donchian Channels)
	BoxData *market.BoxData `json:"box_data,omitempty"`

	// Grid direction (neutral, long, short, long_bias, short_bias)
	CurrentDirection string `json:"current_direction,omitempty"`

	// Macro data for commodity/forex assets (gold, silver, etc.)
	MacroData *macro.GoldMacroData `json:"macro_data,omitempty"`

	// Whether this is an xyz dex asset (gold, silver, forex, stocks)
	IsXyzAsset bool `json:"is_xyz_asset,omitempty"`
}

// ============================================================================
// Grid Volatility Profile Defaults
// ============================================================================

// GridPromptThresholds holds resolved thresholds for AI prompt market regime assessment
type GridPromptThresholds struct {
	RangingBBWidth  float64
	TrendingBBWidth float64
	RangingEMADist  float64
	TrendingEMADist float64
}

// ResolveGridPromptThresholds resolves configured prompt thresholds with defaults for zero values
func ResolveGridPromptThresholds(config *store.GridStrategyConfig) GridPromptThresholds {
	t := GridPromptThresholds{
		RangingBBWidth:  3.0,
		TrendingBBWidth: 4.0,
		RangingEMADist:  1.0,
		TrendingEMADist: 2.0,
	}
	if config == nil {
		return t
	}
	if config.RangingBBWidth > 0 {
		t.RangingBBWidth = config.RangingBBWidth
	}
	if config.TrendingBBWidth > 0 {
		t.TrendingBBWidth = config.TrendingBBWidth
	}
	if config.RangingEMADist > 0 {
		t.RangingEMADist = config.RangingEMADist
	}
	if config.TrendingEMADist > 0 {
		t.TrendingEMADist = config.TrendingEMADist
	}
	return t
}

// ============================================================================
// Grid Prompt Building
// ============================================================================

// BuildGridSystemPrompt builds the system prompt for grid trading AI
// IsGoldAsset detects if the symbol is a gold-related asset.
func IsGoldAsset(symbol string) bool {
	s := strings.ToUpper(symbol)
	s = strings.TrimPrefix(s, "XYZ:")
	for _, suffix := range []string{"USDT", "USD", "-USDC", "-USD"} {
		s = strings.TrimSuffix(s, suffix)
	}
	return s == "GOLD" || s == "XAUUSD" || s == "XAU"
}

// IsXyzAsset detects if the symbol is an xyz dex asset (commodities, stocks, forex).
func IsXyzAsset(symbol string) bool {
	return strings.Contains(strings.ToLower(symbol), "xyz:") || IsGoldAsset(symbol)
}

func BuildGridSystemPrompt(config *store.GridStrategyConfig, lang string) string {
	t := ResolveGridPromptThresholds(config)
	logger.Debugf("[Grid] Prompt thresholds: ranging BB<%.1f%% EMA<%.1f%%, trending BB>%.1f%% EMA>%.1f%% (lang=%s)",
		t.RangingBBWidth, t.RangingEMADist, t.TrendingBBWidth, t.TrendingEMADist, lang)
	if lang == "zh" {
		base := buildGridSystemPromptZh(config)
		if IsGoldAsset(config.Symbol) {
			return base + goldSystemPromptZh()
		}
		return base
	}
	base := buildGridSystemPromptEn(config)
	if IsGoldAsset(config.Symbol) {
		return base + goldSystemPromptEn()
	}
	return base
}

func buildGridSystemPromptZh(config *store.GridStrategyConfig) string {
	t := ResolveGridPromptThresholds(config)
	perLevelNotional := config.TotalInvestment * float64(config.Leverage) / float64(config.GridCount)
	return fmt.Sprintf(`# 你是一个专业的网格交易AI

## 角色定义
你是一个经验丰富的网格交易专家，负责管理 %s 的网格交易策略。你的任务是：
1. 判断当前市场状态（震荡/趋势/高波动）
2. 决定是否需要调整网格或暂停交易
3. 管理每个网格层级的订单

## 网格配置
- 交易对: %s
- 网格层数: %d
- 总投资: %.2f USDT
- 杠杆: %dx
- 价格分布: %s
- 每层名义金额: ~$%.0f (总投资×杠杆÷层数)

## 决策规则

### 市场状态判断
- **震荡市场** (适合网格): 布林带宽度 < %.1f%%, EMA20/50 距离 < %.1f%%, 价格在布林带中轨附近
- **趋势市场** (暂停网格): 布林带宽度 > %.1f%%, EMA20/50 距离 > %.1f%%, 价格持续突破布林带
- **高波动市场** (谨慎): ATR异常放大, 价格剧烈波动

### 网格初始化规则
当所有层级都为 empty 状态时，应按以下逻辑铺单：
- 当前价格**以下**的层级: 下 place_buy_limit（等价格回落买入）
- 当前价格**以上**的层级: 下 place_sell_limit（等价格上涨卖出）
- 距离当前价格最近的 1-2 层可优先下单
- 如果市场正处于趋势状态，先 hold 观望再铺单

### 下单数量计算
每层下单数量 = 每层名义金额 ÷ 该层价格
例如: 名义金额 $%.0f ÷ 价格 $5000 = 数量 %.4f

### 可执行的操作
- place_buy_limit: 在指定价格下买入限价单
- place_sell_limit: 在指定价格下卖出限价单
- cancel_order: 取消指定订单
- cancel_all_orders: 取消所有订单
- pause_grid: 暂停网格交易（趋势市场时）
- resume_grid: 恢复网格交易（震荡市场时）
- adjust_grid: 调整网格边界
- hold: 保持当前状态不操作

## 输出格式
输出JSON数组，每个决策包含:
- symbol: 交易对
- action: 操作类型
- price: 价格（限价单用）
- quantity: 数量（按每层名义金额÷价格计算）
- level_index: 网格层级索引
- order_id: 订单ID（取消订单用）
- confidence: 置信度 0-100
- reasoning: 决策理由

示例:
[
  {"symbol": "%s", "action": "place_buy_limit", "price": %s, "quantity": %s, "level_index": 2, "confidence": 85, "reasoning": "第2层价格接近，下买单"},
  {"symbol": "%s", "action": "hold", "confidence": 90, "reasoning": "市场震荡，保持当前网格"}
]
`, config.Symbol, config.Symbol, config.GridCount, config.TotalInvestment, config.Leverage, config.Distribution,
		perLevelNotional,
		t.RangingBBWidth, t.RangingEMADist, t.TrendingBBWidth, t.TrendingEMADist,
		perLevelNotional, perLevelNotional/5000,
		config.Symbol, gridExamplePrice(config), gridExampleQty(config),
		config.Symbol)
}

func buildGridSystemPromptEn(config *store.GridStrategyConfig) string {
	t := ResolveGridPromptThresholds(config)
	perLevelNotional := config.TotalInvestment * float64(config.Leverage) / float64(config.GridCount)
	return fmt.Sprintf(`# You are a Professional Grid Trading AI

## Role Definition
You are an experienced grid trading expert managing a grid strategy for %s. Your tasks are:
1. Assess current market regime (ranging/trending/volatile)
2. Decide whether to adjust grid or pause trading
3. Manage orders at each grid level

## Grid Configuration
- Symbol: %s
- Grid Levels: %d
- Total Investment: %.2f USDT
- Leverage: %dx
- Distribution: %s
- Per-Level Notional: ~$%.0f (investment × leverage ÷ levels)

## Decision Rules

### Market Regime Assessment
- **Ranging Market** (ideal for grid): Bollinger width < %.1f%%, EMA20/50 distance < %.1f%%, price near middle band
- **Trending Market** (pause grid): Bollinger width > %.1f%%, EMA20/50 distance > %.1f%%, price breaking bands
- **High Volatility** (caution): ATR spike, erratic price movement

### Grid Initialization Rules
When all levels are in empty state, place orders as follows:
- Levels **below** current price: place_buy_limit (buy on dip)
- Levels **above** current price: place_sell_limit (sell on rise)
- Prioritize the 1-2 levels closest to current price
- If market is trending, hold and observe before placing orders

### Order Quantity Calculation
Per-level quantity = per-level notional ÷ level price
Example: $%.0f ÷ $5000 = %.4f

### Available Actions
- place_buy_limit: Place buy limit order at specified price
- place_sell_limit: Place sell limit order at specified price
- cancel_order: Cancel specific order
- cancel_all_orders: Cancel all orders
- pause_grid: Pause grid trading (in trending market)
- resume_grid: Resume grid trading (in ranging market)
- adjust_grid: Adjust grid boundaries
- hold: Maintain current state

## Output Format
Output JSON array, each decision contains:
- symbol: Trading pair
- action: Action type
- price: Price (for limit orders)
- quantity: Quantity (calculated as per-level notional ÷ price)
- level_index: Grid level index
- order_id: Order ID (for cancel)
- confidence: Confidence 0-100
- reasoning: Decision reason

Example:
[
  {"symbol": "%s", "action": "place_buy_limit", "price": %s, "quantity": %s, "level_index": 2, "confidence": 85, "reasoning": "Level 2 price approaching, place buy order"},
  {"symbol": "%s", "action": "hold", "confidence": 90, "reasoning": "Market ranging, maintain current grid"}
]
`, config.Symbol, config.Symbol, config.GridCount, config.TotalInvestment, config.Leverage, config.Distribution,
		perLevelNotional,
		t.RangingBBWidth, t.RangingEMADist, t.TrendingBBWidth, t.TrendingEMADist,
		perLevelNotional, perLevelNotional/5000,
		config.Symbol, gridExamplePrice(config), gridExampleQty(config),
		config.Symbol)
}

// gridExamplePrice returns a realistic example price string based on the asset type.
func gridExamplePrice(config *store.GridStrategyConfig) string {
	if IsGoldAsset(config.Symbol) {
		return "4950"
	}
	return "94000"
}

// gridExampleQty returns a realistic example quantity string based on the asset type.
func gridExampleQty(config *store.GridStrategyConfig) string {
	perLevel := config.TotalInvestment * float64(config.Leverage) / float64(config.GridCount)
	if IsGoldAsset(config.Symbol) {
		return fmt.Sprintf("%.4f", perLevel/5000)
	}
	return fmt.Sprintf("%.4f", perLevel/94000)
}

// ── Gold-Specific System Prompt Extensions ──────────────────────────────────

func goldSystemPromptZh() string {
	return `
## 黄金交易专业知识

### 黄金特有的宏观驱动因子
你在做网格决策时，**必须**结合以下宏观数据（如果提供）:
1. **美元指数 (DXY)**: 黄金以美元计价，DXY上涨→金价下跌，反之亦然。相关系数约-0.8。DXY日涨幅>0.5%时应偏空
2. **美国实际利率 (10Y国债收益率)**: 收益率上升→持有黄金机会成本增加→金价承压。收益率单日上行>5bp应偏空
3. **VIX恐慌指数**: VIX>25→市场恐慌→避险买金，利多；VIX<15→风险偏好强→金价承压
4. **USD/JPY**: 日元与黄金同为避险资产，USD/JPY下跌(日元走强)→通常金价上涨
5. **美股(S&P500/NASDAQ)**: 股市暴跌→避险资金流入黄金；但温和下跌可能两者同跌(流动性紧缩)
6. **COMEX持仓量**: 持仓量骤增→大资金进场，趋势可能延续；持仓量下降→兴趣减弱

### 黄金波动特征
- 黄金日波动通常 0.5-1.5%，远小于加密货币
- 伦敦/纽约交叉时段(北京时间20:00-01:00)波动最大
- 亚盘(北京时间09:00-15:00)通常低波动，适合网格

### 黄金网格决策要点
- 当DXY+美债收益率同时上行 → 强烈建议 pause_grid 或收窄网格
- 当VIX飙升+美股暴跌 → 金价可能单边上涨，建议 pause_grid 等突破结束
- 当宏观面中性+技术面震荡 → 最佳网格运行环境，可适当放宽网格间距
- 非农数据、CPI、FOMC会议前后，建议暂停网格或降低仓位
- 注意: 对于黄金，忽略"资金费率"和"Binance持仓量"指标(不适用)
`
}

func goldSystemPromptEn() string {
	return `
## Gold Trading Expertise

### Gold-Specific Macro Drivers
When making grid decisions, you **MUST** incorporate these macro indicators (if provided):
1. **DXY (US Dollar Index)**: Gold is priced in USD. DXY up → gold down (correlation ~-0.8). DXY daily gain >0.5% → lean bearish
2. **US 10Y Treasury Yield**: Rising yields → higher opportunity cost of holding gold → bearish pressure. Daily rise >5bp → lean bearish
3. **VIX (Fear Index)**: VIX >25 → panic → flight to gold (bullish). VIX <15 → risk-on → gold bearish
4. **USD/JPY**: JPY and gold are both safe havens. USD/JPY falling (JPY strengthening) → usually gold bullish
5. **US Equities (S&P500/NASDAQ)**: Crash → safe-haven inflows to gold; but mild decline may hurt both (liquidity drain)
6. **COMEX Open Interest**: OI surge → big money entering, trend may continue; OI declining → interest fading

### Gold Volatility Characteristics
- Gold daily volatility is typically 0.5-1.5%, far less than crypto
- London/NY crossover session (20:00-01:00 Beijing time) has highest volatility
- Asian session (09:00-15:00 Beijing time) is typically low volatility, ideal for grid

### Gold Grid Decision Guidelines
- When DXY + yields both rising → strongly recommend pause_grid or narrow grid range
- When VIX spiking + equities crashing → gold may run one-directional, recommend pause_grid until breakout settles
- When macro neutral + technicals ranging → best grid environment, can widen grid spacing
- Around NFP, CPI, FOMC → recommend pausing grid or reducing position size
- Note: Ignore "Funding Rate" and "Binance OI" indicators for gold (not applicable)
`
}

// BuildGridUserPrompt builds the user prompt with current grid context
func BuildGridUserPrompt(ctx *GridContext, lang string) string {
	if lang == "zh" {
		return buildGridUserPromptZh(ctx)
	}
	return buildGridUserPromptEn(ctx)
}

func buildGridUserPromptZh(ctx *GridContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## 当前时间: %s\n\n", ctx.CurrentTime))

	// Market data section
	sb.WriteString("## 市场数据\n")
	sb.WriteString(fmt.Sprintf("- 当前价格: $%.2f\n", ctx.CurrentPrice))
	sb.WriteString(fmt.Sprintf("- 1小时涨跌: %.2f%%\n", ctx.PriceChange1h))
	sb.WriteString(fmt.Sprintf("- 4小时涨跌: %.2f%%\n", ctx.PriceChange4h))
	sb.WriteString(fmt.Sprintf("- ATR14: $%.2f (%.2f%%)\n", ctx.ATR14, ctx.ATR14/ctx.CurrentPrice*100))
	sb.WriteString(fmt.Sprintf("- 布林带: 上轨 $%.2f, 中轨 $%.2f, 下轨 $%.2f\n", ctx.BollingerUpper, ctx.BollingerMiddle, ctx.BollingerLower))
	sb.WriteString(fmt.Sprintf("- 布林带宽度: %.2f%%\n", ctx.BollingerWidth))
	sb.WriteString(fmt.Sprintf("- EMA20: $%.2f, EMA50: $%.2f, 距离: %.2f%%\n", ctx.EMA20, ctx.EMA50, ctx.EMADistance))
	sb.WriteString(fmt.Sprintf("- RSI14: %.1f\n", ctx.RSI14))
	sb.WriteString(fmt.Sprintf("- MACD: %.4f, Signal: %.4f, Histogram: %.4f\n", ctx.MACD, ctx.MACDSignal, ctx.MACDHistogram))
	if !ctx.IsXyzAsset {
		sb.WriteString(fmt.Sprintf("- 资金费率: %.4f%%\n", ctx.FundingRate*100))
	}
	sb.WriteString("\n")

	// Macro data section (for gold/commodity assets)
	if ctx.MacroData != nil {
		sb.WriteString(ctx.MacroData.FormatForPromptZh())
	}

	// Box Indicator Section
	if ctx.BoxData != nil {
		sb.WriteString("## 箱体指标 (唐奇安通道)\n\n")
		sb.WriteString("| 箱体级别 | 上轨 | 下轨 | 宽度 |\n")
		sb.WriteString("|----------|------|------|------|\n")

		shortWidth := 0.0
		midWidth := 0.0
		longWidth := 0.0

		if ctx.BoxData.CurrentPrice > 0 {
			shortWidth = (ctx.BoxData.ShortUpper - ctx.BoxData.ShortLower) / ctx.BoxData.CurrentPrice * 100
			midWidth = (ctx.BoxData.MidUpper - ctx.BoxData.MidLower) / ctx.BoxData.CurrentPrice * 100
			longWidth = (ctx.BoxData.LongUpper - ctx.BoxData.LongLower) / ctx.BoxData.CurrentPrice * 100
		}

		sb.WriteString(fmt.Sprintf("| 短期 (3天) | %.2f | %.2f | %.2f%% |\n",
			ctx.BoxData.ShortUpper, ctx.BoxData.ShortLower, shortWidth))
		sb.WriteString(fmt.Sprintf("| 中期 (10天) | %.2f | %.2f | %.2f%% |\n",
			ctx.BoxData.MidUpper, ctx.BoxData.MidLower, midWidth))
		sb.WriteString(fmt.Sprintf("| 长期 (21天) | %.2f | %.2f | %.2f%% |\n",
			ctx.BoxData.LongUpper, ctx.BoxData.LongLower, longWidth))

		sb.WriteString(fmt.Sprintf("\n当前价格: %.2f\n", ctx.BoxData.CurrentPrice))

		// Check position relative to boxes
		price := ctx.BoxData.CurrentPrice
		if price > ctx.BoxData.LongUpper || price < ctx.BoxData.LongLower {
			sb.WriteString("⚠️ 突破: 价格突破长期箱体!\n")
		} else if price > ctx.BoxData.MidUpper || price < ctx.BoxData.MidLower {
			sb.WriteString("⚠️ 警告: 价格接近长期箱体边界\n")
		}
		sb.WriteString("\n")
	}

	// Account section
	sb.WriteString("## 账户状态\n")
	sb.WriteString(fmt.Sprintf("- 总权益: $%.2f\n", ctx.TotalEquity))
	sb.WriteString(fmt.Sprintf("- 可用余额: $%.2f\n", ctx.AvailableBalance))
	sb.WriteString(fmt.Sprintf("- 当前持仓: %.4f (净头寸)\n", ctx.CurrentPosition))
	sb.WriteString(fmt.Sprintf("- 未实现盈亏: $%.2f\n", ctx.UnrealizedPnL))
	sb.WriteString("\n")

	// Grid state section
	sb.WriteString("## 网格状态\n")
	sb.WriteString(fmt.Sprintf("- 网格范围: $%.2f - $%.2f\n", ctx.LowerPrice, ctx.UpperPrice))
	sb.WriteString(fmt.Sprintf("- 网格间距: $%.2f\n", ctx.GridSpacing))
	sb.WriteString(fmt.Sprintf("- 活跃订单数: %d\n", ctx.ActiveOrderCount))
	sb.WriteString(fmt.Sprintf("- 已成交层数: %d\n", ctx.FilledLevelCount))
	sb.WriteString(fmt.Sprintf("- 网格已暂停: %v\n", ctx.IsPaused))
	if ctx.CurrentDirection != "" {
		directionDescZh := map[string]string{
			"neutral":    "中性 (50%买+50%卖)",
			"long":       "做多 (100%买)",
			"short":      "做空 (100%卖)",
			"long_bias":  "偏多 (70%买+30%卖)",
			"short_bias": "偏空 (30%买+70%卖)",
		}
		desc := directionDescZh[ctx.CurrentDirection]
		if desc == "" {
			desc = ctx.CurrentDirection
		}
		sb.WriteString(fmt.Sprintf("- 网格方向: %s\n", desc))
	}
	sb.WriteString("\n")

	// Grid levels detail
	sb.WriteString("## 网格层级详情\n")
	sb.WriteString("| 层级 | 价格 | 状态 | 方向 | 订单数量 | 持仓数量 | 未实现盈亏 |\n")
	sb.WriteString("|------|------|------|------|----------|----------|------------|\n")
	for _, level := range ctx.Levels {
		sb.WriteString(fmt.Sprintf("| %d | $%.2f | %s | %s | %.4f | %.4f | $%.2f |\n",
			level.Index, level.Price, level.State, level.Side,
			level.OrderQuantity, level.PositionSize, level.UnrealizedPnL))
	}
	sb.WriteString("\n")

	// Performance section
	sb.WriteString("## 绩效统计\n")
	sb.WriteString(fmt.Sprintf("- 总利润: $%.2f\n", ctx.TotalProfit))
	sb.WriteString(fmt.Sprintf("- 总交易次数: %d\n", ctx.TotalTrades))
	sb.WriteString(fmt.Sprintf("- 胜率: %.1f%%\n", float64(ctx.WinningTrades)/float64(max(ctx.TotalTrades, 1))*100))
	sb.WriteString(fmt.Sprintf("- 最大回撤: %.2f%%\n", ctx.MaxDrawdown))
	sb.WriteString(fmt.Sprintf("- 今日盈亏: $%.2f\n", ctx.DailyPnL))
	sb.WriteString("\n")

	sb.WriteString("## 请分析以上数据，做出网格交易决策\n")
	sb.WriteString("输出JSON数组格式的决策列表。\n")

	return sb.String()
}

func buildGridUserPromptEn(ctx *GridContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Current Time: %s\n\n", ctx.CurrentTime))

	// Market data section
	sb.WriteString("## Market Data\n")
	sb.WriteString(fmt.Sprintf("- Current Price: $%.2f\n", ctx.CurrentPrice))
	sb.WriteString(fmt.Sprintf("- 1h Change: %.2f%%\n", ctx.PriceChange1h))
	sb.WriteString(fmt.Sprintf("- 4h Change: %.2f%%\n", ctx.PriceChange4h))
	sb.WriteString(fmt.Sprintf("- ATR14: $%.2f (%.2f%%)\n", ctx.ATR14, ctx.ATR14/ctx.CurrentPrice*100))
	sb.WriteString(fmt.Sprintf("- Bollinger Bands: Upper $%.2f, Middle $%.2f, Lower $%.2f\n", ctx.BollingerUpper, ctx.BollingerMiddle, ctx.BollingerLower))
	sb.WriteString(fmt.Sprintf("- Bollinger Width: %.2f%%\n", ctx.BollingerWidth))
	sb.WriteString(fmt.Sprintf("- EMA20: $%.2f, EMA50: $%.2f, Distance: %.2f%%\n", ctx.EMA20, ctx.EMA50, ctx.EMADistance))
	sb.WriteString(fmt.Sprintf("- RSI14: %.1f\n", ctx.RSI14))
	sb.WriteString(fmt.Sprintf("- MACD: %.4f, Signal: %.4f, Histogram: %.4f\n", ctx.MACD, ctx.MACDSignal, ctx.MACDHistogram))
	if !ctx.IsXyzAsset {
		sb.WriteString(fmt.Sprintf("- Funding Rate: %.4f%%\n", ctx.FundingRate*100))
	}
	sb.WriteString("\n")

	// Macro data section (for gold/commodity assets)
	if ctx.MacroData != nil {
		sb.WriteString(ctx.MacroData.FormatForPromptEn())
	}

	// Box Indicator Section
	if ctx.BoxData != nil {
		sb.WriteString("## Box Indicators (Donchian Channels)\n\n")
		sb.WriteString("| Box Level | Upper | Lower | Width |\n")
		sb.WriteString("|-----------|-------|-------|-------|\n")

		shortWidth := 0.0
		midWidth := 0.0
		longWidth := 0.0

		if ctx.BoxData.CurrentPrice > 0 {
			shortWidth = (ctx.BoxData.ShortUpper - ctx.BoxData.ShortLower) / ctx.BoxData.CurrentPrice * 100
			midWidth = (ctx.BoxData.MidUpper - ctx.BoxData.MidLower) / ctx.BoxData.CurrentPrice * 100
			longWidth = (ctx.BoxData.LongUpper - ctx.BoxData.LongLower) / ctx.BoxData.CurrentPrice * 100
		}

		sb.WriteString(fmt.Sprintf("| Short (3d) | %.2f | %.2f | %.2f%% |\n",
			ctx.BoxData.ShortUpper, ctx.BoxData.ShortLower, shortWidth))
		sb.WriteString(fmt.Sprintf("| Mid (10d) | %.2f | %.2f | %.2f%% |\n",
			ctx.BoxData.MidUpper, ctx.BoxData.MidLower, midWidth))
		sb.WriteString(fmt.Sprintf("| Long (21d) | %.2f | %.2f | %.2f%% |\n",
			ctx.BoxData.LongUpper, ctx.BoxData.LongLower, longWidth))

		sb.WriteString(fmt.Sprintf("\nCurrent Price: %.2f\n", ctx.BoxData.CurrentPrice))

		// Check position relative to boxes
		price := ctx.BoxData.CurrentPrice
		if price > ctx.BoxData.LongUpper || price < ctx.BoxData.LongLower {
			sb.WriteString("⚠️ BREAKOUT: Price outside long-term box!\n")
		} else if price > ctx.BoxData.MidUpper || price < ctx.BoxData.MidLower {
			sb.WriteString("⚠️ WARNING: Price approaching long-term box boundary\n")
		}
		sb.WriteString("\n")
	}

	// Account section
	sb.WriteString("## Account Status\n")
	sb.WriteString(fmt.Sprintf("- Total Equity: $%.2f\n", ctx.TotalEquity))
	sb.WriteString(fmt.Sprintf("- Available Balance: $%.2f\n", ctx.AvailableBalance))
	sb.WriteString(fmt.Sprintf("- Current Position: %.4f (net)\n", ctx.CurrentPosition))
	sb.WriteString(fmt.Sprintf("- Unrealized PnL: $%.2f\n", ctx.UnrealizedPnL))
	sb.WriteString("\n")

	// Grid state section
	sb.WriteString("## Grid Status\n")
	sb.WriteString(fmt.Sprintf("- Grid Range: $%.2f - $%.2f\n", ctx.LowerPrice, ctx.UpperPrice))
	sb.WriteString(fmt.Sprintf("- Grid Spacing: $%.2f\n", ctx.GridSpacing))
	sb.WriteString(fmt.Sprintf("- Active Orders: %d\n", ctx.ActiveOrderCount))
	sb.WriteString(fmt.Sprintf("- Filled Levels: %d\n", ctx.FilledLevelCount))
	sb.WriteString(fmt.Sprintf("- Grid Paused: %v\n", ctx.IsPaused))
	if ctx.CurrentDirection != "" {
		directionDescEn := map[string]string{
			"neutral":    "Neutral (50% buy + 50% sell)",
			"long":       "Long (100% buy)",
			"short":      "Short (100% sell)",
			"long_bias":  "Long Bias (70% buy + 30% sell)",
			"short_bias": "Short Bias (30% buy + 70% sell)",
		}
		desc := directionDescEn[ctx.CurrentDirection]
		if desc == "" {
			desc = ctx.CurrentDirection
		}
		sb.WriteString(fmt.Sprintf("- Grid Direction: %s\n", desc))
	}
	sb.WriteString("\n")

	// Grid levels detail
	sb.WriteString("## Grid Levels Detail\n")
	sb.WriteString("| Level | Price | State | Side | Order Qty | Position | Unrealized PnL |\n")
	sb.WriteString("|-------|-------|-------|------|-----------|----------|----------------|\n")
	for _, level := range ctx.Levels {
		sb.WriteString(fmt.Sprintf("| %d | $%.2f | %s | %s | %.4f | %.4f | $%.2f |\n",
			level.Index, level.Price, level.State, level.Side,
			level.OrderQuantity, level.PositionSize, level.UnrealizedPnL))
	}
	sb.WriteString("\n")

	// Performance section
	sb.WriteString("## Performance Stats\n")
	sb.WriteString(fmt.Sprintf("- Total Profit: $%.2f\n", ctx.TotalProfit))
	sb.WriteString(fmt.Sprintf("- Total Trades: %d\n", ctx.TotalTrades))
	sb.WriteString(fmt.Sprintf("- Win Rate: %.1f%%\n", float64(ctx.WinningTrades)/float64(max(ctx.TotalTrades, 1))*100))
	sb.WriteString(fmt.Sprintf("- Max Drawdown: %.2f%%\n", ctx.MaxDrawdown))
	sb.WriteString(fmt.Sprintf("- Daily PnL: $%.2f\n", ctx.DailyPnL))
	sb.WriteString("\n")

	sb.WriteString("## Please analyze the data above and make grid trading decisions\n")
	sb.WriteString("Output a JSON array of decisions.\n")

	return sb.String()
}

// ============================================================================
// Grid Decision Functions
// ============================================================================

// GetGridDecisions gets AI decisions for grid trading
func GetGridDecisions(ctx *GridContext, mcpClient mcp.AIClient, config *store.GridStrategyConfig, lang string) (*FullDecision, error) {
	startTime := time.Now()

	// Build prompts
	systemPrompt := BuildGridSystemPrompt(config, lang)
	userPrompt := BuildGridUserPrompt(ctx, lang)

	logger.Infof("🤖 [Grid] Calling AI for grid decisions...")

	// Call AI
	response, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI call failed: %w", err)
	}

	// Parse decisions from response
	decisions, err := parseGridDecisions(response, ctx.Symbol)
	if err != nil {
		logger.Warnf("Failed to parse grid decisions: %v", err)
		// Return hold decision as fallback
		decisions = []Decision{{
			Symbol:     ctx.Symbol,
			Action:     "hold",
			Confidence: 50,
			Reasoning:  "Failed to parse AI response, holding current state",
		}}
	}

	duration := time.Since(startTime).Milliseconds()
	logger.Infof("⏱️ [Grid] AI call duration: %d ms, decisions: %d", duration, len(decisions))

	// Extract chain of thought from response
	cotTrace := extractCoTTrace(response)

	return &FullDecision{
		SystemPrompt:        systemPrompt,
		UserPrompt:          userPrompt,
		CoTTrace:            cotTrace,
		Decisions:           decisions,
		RawResponse:         response,
		AIRequestDurationMs: duration,
		Timestamp:           time.Now(),
	}, nil
}

// parseGridDecisions parses AI response into grid decisions
func parseGridDecisions(response string, symbol string) ([]Decision, error) {
	// Try to find JSON array in response
	jsonStr := extractJSONArray(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON array found in response")
	}

	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonStr), &decisions); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate and set default symbol
	for i := range decisions {
		if decisions[i].Symbol == "" {
			decisions[i].Symbol = symbol
		}
		// Validate action
		if !isValidGridAction(decisions[i].Action) {
			logger.Warnf("Invalid grid action: %s", decisions[i].Action)
		}
	}

	return decisions, nil
}

// extractJSONArray extracts JSON array from AI response
func extractJSONArray(response string) string {
	// Try to find ```json code block first
	matches := reJSONFence.FindStringSubmatch(response)
	if len(matches) > 1 {
		return matches[1]
	}

	// Try to find raw JSON array
	matches = reJSONArray.FindStringSubmatch(response)
	if len(matches) > 0 {
		return matches[0]
	}

	return ""
}

// isValidGridAction checks if action is a valid grid action
func isValidGridAction(action string) bool {
	validActions := map[string]bool{
		"place_buy_limit":   true,
		"place_sell_limit":  true,
		"cancel_order":      true,
		"cancel_all_orders": true,
		"pause_grid":        true,
		"resume_grid":       true,
		"adjust_grid":       true,
		"hold":              true,
		// Also support standard actions for compatibility
		"open_long":  true,
		"open_short": true,
		"close_long": true,
		"close_short": true,
	}
	return validActions[action]
}

// ============================================================================
// Grid Context Builder Helpers
// ============================================================================

// BuildGridContextFromMarketData builds grid context from market data
func BuildGridContextFromMarketData(mktData *market.Data, config *store.GridStrategyConfig) *GridContext {
	ctx := &GridContext{
		Symbol:       config.Symbol,
		CurrentTime:  time.Now().Format("2006-01-02 15:04:05"),
		CurrentPrice: mktData.CurrentPrice,

		// Grid config
		GridCount:       config.GridCount,
		TotalInvestment: config.TotalInvestment,
		Leverage:        config.Leverage,
		Distribution:    config.Distribution,

		// Market data
		PriceChange1h: mktData.PriceChange1h,
		PriceChange4h: mktData.PriceChange4h,
		FundingRate:   mktData.FundingRate,
	}

	// Extract indicators from timeframe data
	if mktData.TimeframeData != nil {
		if tf5m, ok := mktData.TimeframeData["5m"]; ok {
			if len(tf5m.BOLLUpper) > 0 {
				ctx.BollingerUpper = tf5m.BOLLUpper[len(tf5m.BOLLUpper)-1]
				ctx.BollingerMiddle = tf5m.BOLLMiddle[len(tf5m.BOLLMiddle)-1]
				ctx.BollingerLower = tf5m.BOLLLower[len(tf5m.BOLLLower)-1]
				if ctx.BollingerMiddle > 0 {
					ctx.BollingerWidth = (ctx.BollingerUpper - ctx.BollingerLower) / ctx.BollingerMiddle * 100
				}
			}
			ctx.ATR14 = tf5m.ATR14
			if len(tf5m.RSI14Values) > 0 {
				ctx.RSI14 = tf5m.RSI14Values[len(tf5m.RSI14Values)-1]
			}
		}
	}

	// Extract longer term context (prefer LongerTermContext, fallback to TimeframeData["4h"])
	if mktData.LongerTermContext != nil {
		if ctx.ATR14 == 0 {
			ctx.ATR14 = mktData.LongerTermContext.ATR14
		}
		ctx.EMA50 = mktData.LongerTermContext.EMA50
	}

	// Fallback: extract EMA50 and ATR14 from TimeframeData["4h"] when LongerTermContext is nil
	// This happens when grid uses GetWithTimeframes() which populates TimeframeData but not LongerTermContext
	if ctx.EMA50 == 0 && mktData.TimeframeData != nil {
		if tf4h, ok := mktData.TimeframeData["4h"]; ok {
			if len(tf4h.EMA50Values) > 0 {
				ctx.EMA50 = tf4h.EMA50Values[len(tf4h.EMA50Values)-1]
			}
			if ctx.ATR14 == 0 {
				ctx.ATR14 = tf4h.ATR14
			}
		}
	}

	ctx.EMA20 = mktData.CurrentEMA20
	ctx.MACD = mktData.CurrentMACD
	ctx.MACDSignal = mktData.CurrentMACDSignal
	ctx.MACDHistogram = mktData.CurrentMACDHist

	// Fallback: extract EMA20 from TimeframeData["5m"] when CurrentEMA20 is zero
	if ctx.EMA20 == 0 && mktData.TimeframeData != nil {
		if tf5m, ok := mktData.TimeframeData["5m"]; ok && len(tf5m.EMA20Values) > 0 {
			ctx.EMA20 = tf5m.EMA20Values[len(tf5m.EMA20Values)-1]
		}
	}

	// Calculate EMA distance
	if ctx.EMA50 > 0 {
		ctx.EMADistance = (ctx.EMA20 - ctx.EMA50) / ctx.EMA50 * 100
	}

	return ctx
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
