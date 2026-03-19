package kernel

import (
	"fmt"
	"math"
	"nofx/market"
	"nofx/provider/nofxos"
	"nofx/store"
	"strings"
	"time"
)

// ============================================================================
// Prompt Building - System Prompt
// ============================================================================

// BuildSystemPrompt builds System Prompt according to strategy configuration
func (e *StrategyEngine) BuildSystemPrompt(accountEquity float64, variant string) string {
	var sb strings.Builder
	riskControl := e.config.RiskControl
	promptSections := e.config.PromptSections

	// 0. Data Dictionary & Schema (ensure AI understands all fields)
	lang := e.GetLanguage()
	schemaPrompt := GetSchemaPrompt(lang)
	sb.WriteString(schemaPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("---\n\n")

	// 1. Role definition (editable)
	if promptSections.RoleDefinition != "" {
		sb.WriteString(promptSections.RoleDefinition)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# You are a professional cryptocurrency trading AI\n\n")
		sb.WriteString("Your task is to make trading decisions based on provided market data.\n\n")
	}

	// 2. Trading mode variant
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "aggressive":
		sb.WriteString("## Mode: Aggressive\n- Prioritize capturing trend breakouts, can build positions in batches when confidence ≥ 70\n- Allow higher positions, but must strictly set stop-loss and explain risk-reward ratio\n\n")
	case "conservative":
		sb.WriteString("## Mode: Conservative\n- Only open positions when multiple signals resonate\n- Prioritize cash preservation, must pause for multiple periods after consecutive losses\n\n")
	case "scalping":
		sb.WriteString("## Mode: Scalping\n- Focus on short-term momentum, smaller profit targets but require quick action\n- If price doesn't move as expected within two bars, immediately reduce position or stop-loss\n\n")
	}

	// 3. Hard constraints (risk control)
	btcEthPosValueRatio := riskControl.BTCETHMaxPositionValueRatio
	if btcEthPosValueRatio <= 0 {
		btcEthPosValueRatio = 5.0
	}
	altcoinPosValueRatio := riskControl.AltcoinMaxPositionValueRatio
	if altcoinPosValueRatio <= 0 {
		altcoinPosValueRatio = 1.0
	}

	sb.WriteString("# Hard Constraints (Risk Control)\n\n")
	sb.WriteString("## CODE ENFORCED (Backend validation, cannot be bypassed):\n")
	sb.WriteString(fmt.Sprintf("- Max Positions: %d coins simultaneously\n", riskControl.MaxPositions))
	sb.WriteString(fmt.Sprintf("- Position Value Limit (Altcoins): max %.0f USDT (= equity %.0f × %.1fx)\n",
		accountEquity*altcoinPosValueRatio, accountEquity, altcoinPosValueRatio))
	sb.WriteString(fmt.Sprintf("- Position Value Limit (BTC/ETH): max %.0f USDT (= equity %.0f × %.1fx)\n",
		accountEquity*btcEthPosValueRatio, accountEquity, btcEthPosValueRatio))
	sb.WriteString(fmt.Sprintf("- Max Margin Usage: ≤%.0f%%\n", riskControl.MaxMarginUsage*100))
	minPos := riskControl.MinPositionSize
	if minPos <= 0 {
		minPos = 12
	}
	sb.WriteString(fmt.Sprintf("- Min Position Size: ≥%.0f USDT\n", minPos))

	btcEthMax := accountEquity * btcEthPosValueRatio
	altcoinMax := accountEquity * altcoinPosValueRatio
	if lang == LangChinese {
		if btcEthMax < minPos {
			sb.WriteString(fmt.Sprintf("- ⚠️ BTC/ETH 最大仓位 (%.0f USDT) < 最小仓位 (%.0f USDT) — 禁止开 BTC/ETH 仓位！\n", btcEthMax, minPos))
		}
		if altcoinMax < minPos {
			sb.WriteString(fmt.Sprintf("- ⚠️ 山寨币最大仓位 (%.0f USDT) < 最小仓位 (%.0f USDT) — 禁止开山寨币仓位！\n", altcoinMax, minPos))
		}
	} else {
		if btcEthMax < minPos {
			sb.WriteString(fmt.Sprintf("- ⚠️ BTC/ETH max position (%.0f USDT) < min position (%.0f USDT) — DO NOT open BTC/ETH positions!\n", btcEthMax, minPos))
		}
		if altcoinMax < minPos {
			sb.WriteString(fmt.Sprintf("- ⚠️ Altcoin max position (%.0f USDT) < min position (%.0f USDT) — DO NOT open altcoin positions!\n", altcoinMax, minPos))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## AI GUIDED (Recommended, you should follow):\n")
	sb.WriteString(fmt.Sprintf("- Trading Leverage: Altcoins max %dx | BTC/ETH max %dx\n",
		riskControl.AltcoinMaxLeverage, riskControl.BTCETHMaxLeverage))
	sb.WriteString(fmt.Sprintf("- Risk-Reward Ratio: ≥1:%.1f (take_profit / stop_loss)\n", riskControl.MinRiskRewardRatio))
	minConf := riskControl.MinConfidence
	sb.WriteString(fmt.Sprintf("- Min Confidence: ≥%d to open position\n", minConf))
	if lang == LangChinese {
		sb.WriteString("  置信度评分指南:\n")
		sb.WriteString("  90+: 多时间框架趋势一致 + 宏观面利好 + 量能/OI确认\n")
		sb.WriteString("  80-89: ≥2个时间框架技术信号清晰 + 宏观面中性\n")
		if minConf < 70 {
			sb.WriteString("  70-79: 单时间框架信号 + 部分矛盾因素\n")
			sb.WriteString(fmt.Sprintf("  %d-69: 动量信号明确但缺乏多框架确认 — 轻仓试探\n", minConf))
		} else {
			sb.WriteString(fmt.Sprintf("  %d-79: 单时间框架信号 + 存在一个矛盾因素\n", minConf))
		}
		sb.WriteString(fmt.Sprintf("  <%d: 不开仓 — 等待更好的机会\n\n", minConf))
	} else {
		sb.WriteString("  Confidence scoring guide:\n")
		sb.WriteString("  90+: Multi-timeframe trend alignment + macro favorable + volume/OI confirmation\n")
		sb.WriteString("  80-89: Clear technical signal on ≥2 timeframes + neutral macro\n")
		if minConf < 70 {
			sb.WriteString("  70-79: Single timeframe signal with partial conflicting factors\n")
			sb.WriteString(fmt.Sprintf("  %d-69: Clear momentum signal but lacks multi-timeframe confirmation — light position\n", minConf))
		} else {
			sb.WriteString(fmt.Sprintf("  %d-79: Single timeframe signal with one conflicting factor\n", minConf))
		}
		sb.WriteString(fmt.Sprintf("  <%d: Do NOT open — wait for better setup\n\n", minConf))
	}

	// Position sizing guidance
	sb.WriteString("## Position Sizing Guidance\n")
	sb.WriteString("Calculate `position_size_usd` based on your confidence and the Position Value Limits above:\n")
	sb.WriteString("- High confidence (≥85): Use 80-100%% of max position value limit\n")
	if minConf < 70 {
		sb.WriteString("- Medium confidence (70-84): Use 50-80%% of max position value limit\n")
		sb.WriteString(fmt.Sprintf("- Low confidence (%d-69): Use 30-50%% of max position value limit (light probe)\n", minConf))
	} else {
		sb.WriteString(fmt.Sprintf("- Medium confidence (%d-84): Use 50-80%% of max position value limit\n", minConf))
	}
	sb.WriteString(fmt.Sprintf("- Example: With equity %.0f and BTC/ETH ratio %.1fx, max is %.0f USDT\n",
		accountEquity, btcEthPosValueRatio, accountEquity*btcEthPosValueRatio))
	sb.WriteString("- **DO NOT** just use available_balance as position_size_usd. Use the Position Value Limits!\n\n")

	// Risk verification (mandatory)
	maxSingleRiskPct := 0.10
	maxRiskUsd := accountEquity * maxSingleRiskPct
	if maxRiskUsd < 0.5 {
		maxRiskUsd = 0.5
	}
	sb.WriteString("## ⚠️ Risk Verification (MANDATORY before output)\n")
	sb.WriteString("For every open position, calculate `risk_usd` using this exact formula:\n")
	sb.WriteString("```\n")
	sb.WriteString("risk_usd = position_size_usd × leverage × abs(entry_price - stop_loss) / entry_price\n")
	sb.WriteString("```\n")
	sb.WriteString(fmt.Sprintf("**CODE ENFORCED**: risk_usd must be ≤ **%.2f USDT** (= equity %.0f × %.0f%% max single-trade risk)\n",
		maxRiskUsd, accountEquity, maxSingleRiskPct*100))
	sb.WriteString("If exceeded → reduce position_size_usd, lower leverage, or tighten stop_loss. Backend REJECTS trades that exceed this.\n")
	sb.WriteString("⚠️ Common LLM arithmetic error: verify your multiplication step by step. (e.g. 0.013 × 370 = 4.81, NOT 0.48)\n\n")

	// 4. Trading frequency (editable)
	if promptSections.TradingFrequency != "" {
		sb.WriteString(promptSections.TradingFrequency)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# ⏱️ Trading Frequency Awareness\n\n")
		sb.WriteString("- Excellent traders: 2-4 trades/day ≈ 0.1-0.2 trades/hour\n")
		sb.WriteString("- >2 trades/hour = Overtrading\n")
		sb.WriteString("- Single position hold time ≥ 30-60 minutes\n")
		sb.WriteString("If you find yourself trading every period → standards too low; if closing positions < 30 minutes → too impatient.\n\n")
	}

	// 5. Entry standards (editable)
	if promptSections.EntryStandards != "" {
		sb.WriteString(promptSections.EntryStandards)
		sb.WriteString("\n\nYou have the following indicator data:\n")
		e.writeAvailableIndicators(&sb)
		sb.WriteString(fmt.Sprintf("\n**Confidence ≥ %d** required to open positions.\n\n", riskControl.MinConfidence))
	} else {
		sb.WriteString("# 🎯 Entry Standards (Strict)\n\n")
		sb.WriteString("Only open positions when multiple signals resonate. You have:\n")
		e.writeAvailableIndicators(&sb)
		sb.WriteString(fmt.Sprintf("\nFeel free to use any effective analysis method, but **confidence ≥ %d** required to open positions; avoid low-quality behaviors such as single indicators, contradictory signals, sideways consolidation, reopening immediately after closing, etc.\n\n", riskControl.MinConfidence))
	}

	// 5.5 Trade Quality Control (anti-whipsaw, entry quality, loss pattern awareness)
	if lang == LangChinese {
		sb.WriteString("# 🛡️ 交易质量控制（防打脸机制）\n\n")

		sb.WriteString("## 入场位置要求\n")
		sb.WriteString("仅凭指标信号（如 RSI 超卖）不构成开仓理由，还必须确认价格位置：\n")
		sb.WriteString("- 做多：价格须在关键支撑附近（布林下轨、EMA 支撑、近期摆动低点）\n")
		sb.WriteString("- 做空：价格须在关键阻力附近（布林上轨、EMA 阻力、近期摆动高点）\n")
		sb.WriteString("- 价格在布林中轨附近 = 无人区，除非有极强趋势信号否则不开仓\n")
		sb.WriteString("- 禁止追涨杀跌：价格已朝你方向大幅运动 → 入场窗口已过，等回踩\n\n")

		sb.WriteString("## 反转冷却（软约束）\n")
		sb.WriteString("刚平仓亏损后在同一币种上反向开仓 = 高风险操作。不禁止，但需满足：\n")
		sb.WriteString("- 置信度扣 15 分（扣后仍须 ≥ 最低置信度门槛）\n")
		sb.WriteString("- 推理中必须回答：为什么这次反转不同于上次失败？有哪些新信号？\n")
		sb.WriteString("- 须有 ≥2 个上次交易中不存在的额外确认信号\n\n")

		sb.WriteString("## 连亏自省\n")
		sb.WriteString("开仓前检查「近期已完成交易」中同一币种的记录：\n")
		sb.WriteString("- 近 3 笔中 ≥2 笔亏损 → 该币种置信度扣 10 分\n")
		sb.WriteString("- 出现 多→空→多 或 空→多→空 交替亏损 → whipsaw 模式，该币种本周期必须 wait\n\n")

		sb.WriteString("## 置信度扣分表（与正面评分叠加）\n")
		sb.WriteString("| 情况 | 扣分 |\n")
		sb.WriteString("|------|------|\n")
		sb.WriteString("| 同币种平仓亏损后反向开仓 | -15 |\n")
		sb.WriteString("| 近 3 笔同币种交易 ≥2 笔亏损 | -10 |\n")
		sb.WriteString("| 价格在布林中轨附近（非关键位） | -10 |\n")
		sb.WriteString("| 仅单一时间框架信号 | -10 |\n")
		sb.WriteString("| 震荡市中非区间边缘入场 | -15 |\n\n")
	} else {
		sb.WriteString("# 🛡️ Trade Quality Control (Anti-Whipsaw)\n\n")

		sb.WriteString("## Entry Position Requirement\n")
		sb.WriteString("Indicator signals alone (e.g. RSI oversold) are NOT sufficient to open. Confirm price position:\n")
		sb.WriteString("- Long: price must be near key support (BOLL lower band, EMA support, recent swing low)\n")
		sb.WriteString("- Short: price must be near key resistance (BOLL upper band, EMA resistance, recent swing high)\n")
		sb.WriteString("- Price near BOLL middle band = no-man's land — do NOT open unless very strong trend signal\n")
		sb.WriteString("- No chasing: if price already moved significantly in your direction → entry window passed, wait for pullback\n\n")

		sb.WriteString("## Reversal Cooldown (Soft Constraint)\n")
		sb.WriteString("Opening reverse position on SAME coin right after closing at a loss = high-risk move. Not banned, but requires:\n")
		sb.WriteString("- Confidence penalty: -15 points (must still meet minimum after deduction)\n")
		sb.WriteString("- Must explain in reasoning: why is this reversal different from the failed trade? What new signals?\n")
		sb.WriteString("- Must have ≥2 additional confirming signals that the previous trade lacked\n\n")

		sb.WriteString("## Loss Pattern Self-Check\n")
		sb.WriteString("Before opening, check Recent Completed Trades for same-coin patterns:\n")
		sb.WriteString("- If ≥2 of last 3 trades on same coin are losses → deduct 10 confidence points\n")
		sb.WriteString("- If alternating long→short→long or short→long→short losses → whipsaw detected → WAIT this cycle\n\n")

		sb.WriteString("## Confidence Deduction Table (stack with positive scoring)\n")
		sb.WriteString("| Condition | Deduction |\n")
		sb.WriteString("|-----------|----------|\n")
		sb.WriteString("| Reverse position after loss on same coin | -15 |\n")
		sb.WriteString("| ≥2 of last 3 trades on same coin are losses | -10 |\n")
		sb.WriteString("| Price near BOLL mid-band (not at key level) | -10 |\n")
		sb.WriteString("| Only single timeframe signal | -10 |\n")
		sb.WriteString("| Ranging market, not at range edge | -15 |\n\n")
	}

	// 6. Decision process (editable)
	if promptSections.DecisionProcess != "" {
		sb.WriteString(promptSections.DecisionProcess)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# 📋 Decision Process\n\n")
		sb.WriteString("1. Check positions → Should we take profit/stop-loss\n")
		sb.WriteString("2. Scan candidate coins + multi-timeframe → Are there strong signals\n")
		sb.WriteString("3. Write chain of thought first, then output structured JSON\n\n")
	}

	// 7. Output format
	sb.WriteString("# Output Format (Strictly Follow)\n\n")
	sb.WriteString("**Must use XML tags <reasoning> and <decision> to separate chain of thought and decision JSON, avoiding parsing errors**\n\n")
	sb.WriteString("## Format Requirements\n\n")
	sb.WriteString("<reasoning>\n")
	sb.WriteString("Your chain of thought analysis...\n")
	sb.WriteString("- Briefly analyze your thinking process \n")
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("Step 2: JSON decision array\n\n")
	sb.WriteString("```json\n[\n")
	// Use the actual configured position value ratio for BTC/ETH in the example
	examplePositionSize := accountEquity * btcEthPosValueRatio
	exampleRiskUsd := accountEquity * 0.03
	if exampleRiskUsd < 0.5 {
		exampleRiskUsd = 0.5
	}
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": %.2f},\n",
		riskControl.BTCETHMaxLeverage, examplePositionSize, exampleRiskUsd))
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\"}\n")
	sb.WriteString("]\n```\n")
	sb.WriteString("</decision>\n\n")
	sb.WriteString("## Field Description\n\n")
	sb.WriteString("- `action`: ONLY these 6 values are valid: open_long | open_short | close_long | close_short | hold | wait\n")
	sb.WriteString("  - `hold` = keep existing position (use reasoning to describe stop-loss/take-profit adjustments)\n")
	sb.WriteString("  - ⚠️ Do NOT invent actions like `modify_stop_loss`, `adjust_tp`, etc. — they will be rejected\n")
	sb.WriteString(fmt.Sprintf("- `confidence`: 0-100 (opening recommended ≥ %d)\n", riskControl.MinConfidence))
	sb.WriteString("- Required when opening: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd\n")
	sb.WriteString("- **IMPORTANT**: All numeric values must be calculated numbers, NOT formulas/expressions (e.g., use `27.76` not `3000 * 0.01`)\n\n")

	// 8. Custom Prompt
	if e.config.CustomPrompt != "" {
		sb.WriteString("# 📌 Personalized Trading Strategy\n\n")
		sb.WriteString(e.config.CustomPrompt)
		sb.WriteString("\n\n")
		sb.WriteString("Note: The above personalized strategy is a supplement to the basic rules and cannot violate the basic risk control principles.\n")
	}

	return sb.String()
}

func (e *StrategyEngine) writeAvailableIndicators(sb *strings.Builder) {
	indicators := e.config.Indicators
	kline := indicators.Klines

	sb.WriteString(fmt.Sprintf("- %s price series", kline.PrimaryTimeframe))
	if kline.EnableMultiTimeframe {
		sb.WriteString(fmt.Sprintf(" + %s K-line series\n", kline.LongerTimeframe))
	} else {
		sb.WriteString("\n")
	}

	if indicators.EnableEMA {
		sb.WriteString("- EMA indicators")
		if len(indicators.EMAPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.EMAPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableMACD {
		sb.WriteString("- MACD indicators\n")
	}

	if indicators.EnableRSI {
		sb.WriteString("- RSI indicators")
		if len(indicators.RSIPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.RSIPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableATR {
		sb.WriteString("- ATR indicators")
		if len(indicators.ATRPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.ATRPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableBOLL {
		sb.WriteString("- Bollinger Bands (BOLL) - Upper/Middle/Lower bands")
		if len(indicators.BOLLPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.BOLLPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableVolume {
		sb.WriteString("- Volume data\n")
	}

	if indicators.EnableOI {
		sb.WriteString("- Open Interest (OI) data\n")
	}

	if indicators.EnableFundingRate {
		sb.WriteString("- Funding rate\n")
	}

	if len(e.config.CoinSource.StaticCoins) > 0 || e.config.CoinSource.UseAI500 || e.config.CoinSource.UseOITop {
		sb.WriteString("- AI500 / OI_Top filter tags (if available)\n")
	}

	if indicators.EnableQuantData {
		sb.WriteString("- Quantitative data (institutional/retail fund flow, position changes, multi-period price changes)\n")
	}

	lang := e.GetLanguage()

	if indicators.EnableMacroData {
		if lang == LangChinese {
			sb.WriteString("- 宏观指标（日内OHLC）：黄金、原油、白银、铜、标普500、纳斯达克、VIX、美国10年期国债、美元指数、美元/日元\n")
			sb.WriteString("- 消息面：近期重要财经新闻标题 — 重点关注国际消息（地缘冲突、美联储/ECB央行政策、大宗商品异动、美股大事件），忽略仅影响中国A股/国内市场的新闻\n")
			sb.WriteString("  定位：宏观面 = 方向过滤器（不做逆宏观方向的交易），技术面 = 入场触发器，消息面 = 风险事件预警\n")
			sb.WriteString("  关键关联：\n")
			sb.WriteString("    DXY↑ → BTC/黄金承压 | DXY↓ → BTC/黄金利好\n")
			sb.WriteString("    VIX>25 → 极度恐慌，避险资金流入（利多黄金，BTC波动加大）\n")
			sb.WriteString("    US10Y↑ → 无息资产持有成本增加（利空黄金/BTC）\n")
			sb.WriteString("    原油飙升 → 通胀风险 → 鹰派加息预期\n")
			sb.WriteString("    标普500/纳斯达克趋势 → 整体风险偏好指标\n")
		} else {
			sb.WriteString("- Macro indicators (intraday OHLC): Gold, Oil, Silver, Copper, S&P500, NASDAQ, VIX, US10Y, DXY, USDJPY\n")
			sb.WriteString("- Market news: recent major financial headlines — focus on INTERNATIONAL events (geopolitics, Fed/ECB policy, commodity shocks, US equity events); ignore news only affecting China A-shares / domestic market\n")
			sb.WriteString("  Role: Macro data = directional FILTER (do NOT trade against macro trend), Technical = entry TRIGGER, News = risk event alert\n")
			sb.WriteString("  Key correlations:\n")
			sb.WriteString("    DXY↑ → BTC/Gold bearish | DXY↓ → BTC/Gold bullish\n")
			sb.WriteString("    VIX>25 → extreme fear, safe-haven flow (bullish Gold, volatile BTC)\n")
			sb.WriteString("    US10Y↑ → higher holding cost for non-yield assets (bearish Gold/BTC)\n")
			sb.WriteString("    Oil spike → inflation risk → hawkish Fed expectation\n")
			sb.WriteString("    S&P500/NASDAQ trend → overall risk appetite gauge\n")
		}
	}

	if kline.EnableMultiTimeframe && len(kline.SelectedTimeframes) > 1 {
		if lang == LangChinese {
			sb.WriteString("\n**多时间框架分析协议：**\n")
			sb.WriteString("- 4H：判断趋势方向 — 只做顺趋势交易（EMA斜率 + RSI位置）\n")
			sb.WriteString("- 1H：确认市场结构 — 识别支撑/阻力/形态\n")
			sb.WriteString("- 15M：精确入场时机 — 寻找回踩完成或突破确认\n")
			sb.WriteString("- 5M：微调止损位 — 使用近期摆动高/低点\n")
			sb.WriteString("- 规则：若4H和1H方向矛盾 → 等待，不强行入场\n")
		} else {
			sb.WriteString("\n**Multi-Timeframe Analysis Protocol:**\n")
			sb.WriteString("- 4H: Determine trend direction — ONLY trade WITH the trend (EMA slope + RSI position)\n")
			sb.WriteString("- 1H: Confirm market structure — identify support/resistance/patterns\n")
			sb.WriteString("- 15M: Refine entry timing — look for pullback completion or breakout confirmation\n")
			sb.WriteString("- 5M: Fine-tune stop loss placement — use recent swing high/low\n")
			sb.WriteString("- Rule: If 4H and 1H disagree on direction → WAIT, do not force entry\n")
		}
	}

	if lang == LangChinese {
		sb.WriteString("\n**市场状态分类（分析时首先判断）：**\n")
		sb.WriteString("- 趋势市：EMA20 > EMA50 且斜率一致，RSI 40-70（上涨）或 30-60（下跌）→ 顺趋势交易\n")
		sb.WriteString("- 震荡市：EMA20 ≈ EMA50（横盘），RSI 在 40-60 之间震荡，ATR 下降\n")
		sb.WriteString("  → 置信度上限 75，只能在布林上下轨附近入场\n")
		sb.WriteString("  → 识别方法：4H 内价格波动 <2% = 震荡。不要在震荡市中追趋势！\n")
		sb.WriteString("  → 如果连续 2 笔在震荡市中亏损 → 停止该币种交易，等待趋势明确\n")
		sb.WriteString("- 高波动：ATR 飙升（>1.5 倍均值），VIX 升高 → 缩小仓位，放宽止损\n")
	} else {
		sb.WriteString("\n**Market Regime Classification (do this FIRST in your analysis):**\n")
		sb.WriteString("- TRENDING: EMA20 > EMA50 with consistent slope, RSI 40-70 (up) or 30-60 (down) → trade with trend\n")
		sb.WriteString("- RANGING: EMA20 ≈ EMA50 (flat), RSI oscillating 40-60, ATR declining\n")
		sb.WriteString("  → Confidence cap at 75, ONLY enter near BOLL upper/lower bands\n")
		sb.WriteString("  → Detection: price range <2% over 4H = ranging. Do NOT chase trends in ranging markets!\n")
		sb.WriteString("  → If 2 consecutive losses in ranging market → stop trading that coin, wait for trend\n")
		sb.WriteString("- HIGH VOLATILITY: ATR spike (>1.5× average), VIX elevated → reduce position size, widen stops\n")
	}
}

// ============================================================================
// Prompt Building - User Prompt
// ============================================================================

// BuildUserPrompt builds User Prompt based on strategy configuration
func (e *StrategyEngine) BuildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// System status
	sb.WriteString(fmt.Sprintf("Time: %s | Period: #%d | Runtime: %d minutes\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC market
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// Macro data (gold, oil, indices, VIX, etc.)
	if ctx.MacroData != nil {
		lang := e.GetLanguage()
		if lang == LangChinese {
			sb.WriteString(ctx.MacroData.FormatForPromptZh())
		} else {
			sb.WriteString(ctx.MacroData.FormatForPromptEn())
		}
	}

	// Account information
	sb.WriteString(fmt.Sprintf("Account: Equity %.2f | Balance %.2f (%.1f%%) | PnL %+.2f%% | Margin %.1f%% | Positions %d\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// Recently completed orders (placed before positions to ensure visibility)
	if len(ctx.RecentOrders) > 0 {
		sb.WriteString("## Recent Completed Trades\n")
		for i, order := range ctx.RecentOrders {
			resultStr := "Profit"
			if order.RealizedPnL < 0 {
				resultStr = "Loss"
			}
			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Exit %.4f | %s: %+.2f USDT (%+.2f%%) | %s→%s (%s)\n",
				i+1, order.Symbol, order.Side,
				order.EntryPrice, order.ExitPrice,
				resultStr, order.RealizedPnL, order.PnLPct,
				order.EntryTime, order.ExitTime, order.HoldDuration))
		}
		sb.WriteString("\n")
	}

	// Historical trading statistics (helps AI understand past performance)
	if ctx.TradingStats != nil && ctx.TradingStats.TotalTrades > 0 {
		lang := e.GetLanguage()
		totalTrades := ctx.TradingStats.TotalTrades
		insufficientData := totalTrades < 5

		var winLossRatio float64
		if ctx.TradingStats.AvgLoss > 0 {
			winLossRatio = ctx.TradingStats.AvgWin / ctx.TradingStats.AvgLoss
		}

		if lang == LangChinese {
			sb.WriteString("## 历史交易统计\n")
			if insufficientData {
				sb.WriteString(fmt.Sprintf("总交易: %d 笔（样本量不足，统计指标仅供参考）\n", totalTrades))
				sb.WriteString(fmt.Sprintf("总盈亏: %+.2f USDT | 平均盈利: +%.2f | 平均亏损: -%.2f\n",
					ctx.TradingStats.TotalPnL, ctx.TradingStats.AvgWin, ctx.TradingStats.AvgLoss))
				sb.WriteString("表现: 数据不足 - 样本量<5笔，无法可靠评估，正常交易即可\n")
			} else {
				sb.WriteString(fmt.Sprintf("总交易: %d 笔 | 盈利因子: %.2f | 夏普比率: %.2f | 盈亏比: %.2f\n",
					totalTrades, ctx.TradingStats.ProfitFactor, ctx.TradingStats.SharpeRatio, winLossRatio))
				sb.WriteString(fmt.Sprintf("总盈亏: %+.2f USDT | 平均盈利: +%.2f | 平均亏损: -%.2f | 最大回撤: %.1f%%\n",
					ctx.TradingStats.TotalPnL, ctx.TradingStats.AvgWin, ctx.TradingStats.AvgLoss, ctx.TradingStats.MaxDrawdownPct))
				if ctx.TradingStats.ProfitFactor >= 1.5 && ctx.TradingStats.SharpeRatio >= 1 {
					sb.WriteString("表现: 良好 - 保持当前策略\n")
				} else if ctx.TradingStats.ProfitFactor < 1 {
					sb.WriteString("表现: 需改进 - 提高盈亏比，优化止盈止损\n")
				} else if ctx.TradingStats.MaxDrawdownPct > 30 {
					sb.WriteString("表现: 风险偏高 - 减少仓位，控制回撤\n")
				} else {
					sb.WriteString("表现: 正常 - 有优化空间\n")
				}
			}
		} else {
			sb.WriteString("## Historical Trading Statistics\n")
			if insufficientData {
				sb.WriteString(fmt.Sprintf("Total Trades: %d (insufficient sample, stats are for reference only)\n", totalTrades))
				sb.WriteString(fmt.Sprintf("Total PnL: %+.2f USDT | Avg Win: +%.2f | Avg Loss: -%.2f\n",
					ctx.TradingStats.TotalPnL, ctx.TradingStats.AvgWin, ctx.TradingStats.AvgLoss))
				sb.WriteString("Performance: INSUFFICIENT DATA - <5 trades, cannot reliably evaluate, trade normally\n")
			} else {
				sb.WriteString(fmt.Sprintf("Total Trades: %d | Profit Factor: %.2f | Sharpe: %.2f | Win/Loss Ratio: %.2f\n",
					totalTrades, ctx.TradingStats.ProfitFactor, ctx.TradingStats.SharpeRatio, winLossRatio))
				sb.WriteString(fmt.Sprintf("Total PnL: %+.2f USDT | Avg Win: +%.2f | Avg Loss: -%.2f | Max Drawdown: %.1f%%\n",
					ctx.TradingStats.TotalPnL, ctx.TradingStats.AvgWin, ctx.TradingStats.AvgLoss, ctx.TradingStats.MaxDrawdownPct))
				if ctx.TradingStats.ProfitFactor >= 1.5 && ctx.TradingStats.SharpeRatio >= 1 {
					sb.WriteString("Performance: GOOD - maintain current strategy\n")
				} else if ctx.TradingStats.ProfitFactor < 1 {
					sb.WriteString("Performance: NEEDS IMPROVEMENT - improve win/loss ratio, optimize TP/SL\n")
				} else if ctx.TradingStats.MaxDrawdownPct > 30 {
					sb.WriteString("Performance: HIGH RISK - reduce position size, control drawdown\n")
				} else {
					sb.WriteString("Performance: NORMAL - room for optimization\n")
				}
			}
		}
		sb.WriteString("\n")
	}

	// Position information
	if len(ctx.Positions) > 0 {
		sb.WriteString("## Current Positions\n")
		for i, pos := range ctx.Positions {
			sb.WriteString(e.formatPositionInfo(i+1, pos, ctx))
		}
	} else {
		sb.WriteString("Current Positions: None\n\n")
	}

	// Candidate coins (exclude coins already in positions to avoid duplicate data)
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		// Normalize symbol to handle both "ETH" and "ETHUSDT" formats
		normalizedSymbol := market.Normalize(pos.Symbol)
		positionSymbols[normalizedSymbol] = true
	}

	sb.WriteString(fmt.Sprintf("## Candidate Coins (%d coins)\n\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		// Skip if this coin is already a position (data already shown in positions section)
		normalizedCoinSymbol := market.Normalize(coin.Symbol)
		if positionSymbols[normalizedCoinSymbol] {
			continue
		}

		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTags := e.formatCoinSourceTag(coin.Sources)
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(e.formatMarketData(marketData))

		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[coin.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Get language for market data formatting
	nofxosLang := nofxos.LangEnglish
	if e.GetLanguage() == LangChinese {
		nofxosLang = nofxos.LangChinese
	}

	// Market-wide ranking data (for sentiment analysis ONLY)
	hasRanking := ctx.OIRankingData != nil || ctx.NetFlowRankingData != nil || ctx.PriceRankingData != nil
	if hasRanking {
		if e.GetLanguage() == LangChinese {
			sb.WriteString("## ⚠️ 全市场排行数据（仅供判断市场情绪，不可据此开仓）\n\n")
			sb.WriteString("以下排行数据来自全市场，包含不在你候选列表中的币种。\n")
			sb.WriteString("**严格规则：你只能对上方 Candidate Coins / Current Positions 中出现的币种做出开仓决策。排行中的其他币种仅用于辅助判断市场整体方向和情绪。**\n\n")
		} else {
			sb.WriteString("## ⚠️ Market-Wide Rankings (Sentiment Reference ONLY — Do NOT Trade)\n\n")
			sb.WriteString("The rankings below cover the ENTIRE market and include coins NOT in your candidate list.\n")
			sb.WriteString("**STRICT RULE: You may ONLY open positions on coins listed in Candidate Coins / Current Positions above. Use ranking data solely to gauge overall market direction and sentiment.**\n\n")
		}

		if ctx.OIRankingData != nil {
			sb.WriteString(nofxos.FormatOIRankingForAI(ctx.OIRankingData, nofxosLang))
		}
		if ctx.NetFlowRankingData != nil {
			sb.WriteString(nofxos.FormatNetFlowRankingForAI(ctx.NetFlowRankingData, nofxosLang))
		}
		if ctx.PriceRankingData != nil {
			sb.WriteString(nofxos.FormatPriceRankingForAI(ctx.PriceRankingData, nofxosLang))
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("Now please analyze and output your decision (Chain of Thought + JSON)\n")

	return sb.String()
}

func (e *StrategyEngine) formatPositionInfo(index int, pos PositionInfo, ctx *Context) string {
	var sb strings.Builder

	holdingDuration := ""
	if pos.UpdateTime > 0 {
		durationMs := time.Now().UnixMilli() - pos.UpdateTime
		durationMin := durationMs / (1000 * 60)
		if durationMin < 60 {
			holdingDuration = fmt.Sprintf(" | Holding Duration %d min", durationMin)
		} else {
			durationHour := durationMin / 60
			durationMinRemainder := durationMin % 60
			holdingDuration = fmt.Sprintf(" | Holding Duration %dh %dm", durationHour, durationMinRemainder)
		}
	}

	positionValue := pos.Quantity * pos.MarkPrice
	if positionValue < 0 {
		positionValue = -positionValue
	}

	sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Current %.4f | Qty %.4f | Position Value %.2f USDT | PnL%+.2f%% | PnL Amount%+.2f USDT | Peak PnL%.2f%% | Leverage %dx | Margin %.0f | Liq Price %.4f%s\n\n",
		index, pos.Symbol, strings.ToUpper(pos.Side),
		pos.EntryPrice, pos.MarkPrice, pos.Quantity, positionValue, pos.UnrealizedPnLPct, pos.UnrealizedPnL, pos.PeakPnLPct,
		pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

	if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
		sb.WriteString(e.formatMarketData(marketData))

		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[pos.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (e *StrategyEngine) formatCoinSourceTag(sources []string) string {
	if len(sources) > 1 {
		// Multiple signal source combination
		hasAI500 := false
		hasOITop := false
		hasOILow := false
		hasHyperAll := false
		hasHyperMain := false
		for _, s := range sources {
			switch s {
			case "ai500":
				hasAI500 = true
			case "oi_top":
				hasOITop = true
			case "oi_low":
				hasOILow = true
			case "hyper_all":
				hasHyperAll = true
			case "hyper_main":
				hasHyperMain = true
			}
		}
		if hasAI500 && hasOITop {
			return " (AI500+OI_Top dual signal)"
		}
		if hasAI500 && hasOILow {
			return " (AI500+OI_Low dual signal)"
		}
		if hasOITop && hasOILow {
			return " (OI_Top+OI_Low)"
		}
		if hasHyperMain && hasAI500 {
			return " (HyperMain+AI500)"
		}
		if hasHyperAll || hasHyperMain {
			return " (Hyperliquid)"
		}
		return " (Multiple sources)"
	} else if len(sources) == 1 {
		switch sources[0] {
		case "ai500":
			return " (AI500)"
		case "oi_top":
			return " (OI_Top OI increase)"
		case "oi_low":
			return " (OI_Low OI decrease)"
		case "static":
			return " (Manual selection)"
		case "hyper_all":
			return " (Hyperliquid All)"
		case "hyper_main":
			return " (Hyperliquid Top20)"
		}
	}
	return ""
}

// ============================================================================
// Market Data Formatting
// ============================================================================

func (e *StrategyEngine) formatMarketData(data *market.Data) string {
	var sb strings.Builder
	indicators := e.config.Indicators

	// Clearly label the coin symbol
	sb.WriteString(fmt.Sprintf("=== %s Market Data ===\n\n", data.Symbol))
	sb.WriteString(fmt.Sprintf("current_price = %.4f\n\n", data.CurrentPrice))

	if indicators.EnableOI || indicators.EnableFundingRate {
		sb.WriteString(fmt.Sprintf("Additional data for %s:\n\n", data.Symbol))

		if indicators.EnableOI && data.OpenInterest != nil {
			sb.WriteString(fmt.Sprintf("Open Interest: Latest: %.2f Average: %.2f\n\n",
				data.OpenInterest.Latest, data.OpenInterest.Average))
		}

		if indicators.EnableFundingRate {
			sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))
		}
	}

	if len(data.TimeframeData) > 0 {
		timeframeOrder := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w"}
		for _, tf := range timeframeOrder {
			if tfData, ok := data.TimeframeData[tf]; ok {
				sb.WriteString(fmt.Sprintf("=== %s Timeframe (oldest → latest) ===\n\n", strings.ToUpper(tf)))
				e.formatTimeframeSeriesData(&sb, tfData, indicators, tf)
			}
		}
	} else {
		// Compatible with old data format
		if data.IntradaySeries != nil {
			klineConfig := indicators.Klines
			sb.WriteString(fmt.Sprintf("Intraday series (%s intervals, oldest → latest):\n\n", klineConfig.PrimaryTimeframe))

			if len(data.IntradaySeries.MidPrices) > 0 {
				sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
			}

			if indicators.EnableEMA && len(data.IntradaySeries.EMA20Values) > 0 {
				sb.WriteString(fmt.Sprintf("EMA indicators (20-period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
			}

			if indicators.EnableMACD && len(data.IntradaySeries.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
			}

			if indicators.EnableRSI {
				if len(data.IntradaySeries.RSI7Values) > 0 {
					sb.WriteString(fmt.Sprintf("RSI indicators (7-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
				}
				if len(data.IntradaySeries.RSI14Values) > 0 {
					sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
				}
			}

			if indicators.EnableVolume && len(data.IntradaySeries.Volume) > 0 {
				sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.IntradaySeries.Volume)))
			}

			if indicators.EnableATR {
				sb.WriteString(fmt.Sprintf("3m ATR (14-period): %.3f\n\n", data.IntradaySeries.ATR14))
			}
		}

		if data.LongerTermContext != nil && indicators.Klines.EnableMultiTimeframe {
			sb.WriteString(fmt.Sprintf("Longer-term context (%s timeframe):\n\n", indicators.Klines.LongerTimeframe))

			if indicators.EnableEMA {
				sb.WriteString(fmt.Sprintf("20-Period EMA: %.3f vs. 50-Period EMA: %.3f\n\n",
					data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))
			}

			if indicators.EnableATR {
				sb.WriteString(fmt.Sprintf("3-Period ATR: %.3f vs. 14-Period ATR: %.3f\n\n",
					data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))
			}

			if indicators.EnableVolume {
				sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
					data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))
			}

			if indicators.EnableMACD && len(data.LongerTermContext.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
			}

			if indicators.EnableRSI && len(data.LongerTermContext.RSI14Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
			}
		}
	}

	return sb.String()
}

func klineDisplayLimit(tf string) int {
	switch tf {
	case "1m", "3m", "5m":
		return 15
	case "15m", "30m":
		return 20
	default:
		return 30
	}
}

func (e *StrategyEngine) formatTimeframeSeriesData(sb *strings.Builder, data *market.TimeframeSeriesData, indicators store.IndicatorConfig, tf string) {
	if len(data.Klines) > 0 {
		klines := data.Klines
		limit := klineDisplayLimit(tf)
		if len(klines) > limit {
			klines = klines[len(klines)-limit:]
		}
		sb.WriteString("Time(UTC)      Open      High      Low       Close     Volume\n")
		for i, k := range klines {
			t := time.Unix(k.Time/1000, 0).UTC()
			timeStr := t.Format("01-02 15:04")
			marker := ""
			if i == len(klines)-1 {
				marker = "  <- current"
			}
			sb.WriteString(fmt.Sprintf("%-14s %-9.4f %-9.4f %-9.4f %-9.4f %-12.2f%s\n",
				timeStr, k.Open, k.High, k.Low, k.Close, k.Volume, marker))
		}
		sb.WriteString("\n")
	} else if len(data.MidPrices) > 0 {
		sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.MidPrices)))
		if indicators.EnableVolume && len(data.Volume) > 0 {
			sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.Volume)))
		}
	}

	var currentPrice float64
	if len(data.Klines) > 0 {
		currentPrice = data.Klines[len(data.Klines)-1].Close
	} else if len(data.MidPrices) > 0 {
		currentPrice = data.MidPrices[len(data.MidPrices)-1]
	}
	writeIndicatorSummary(sb, data, indicators, currentPrice)
}

func (e *StrategyEngine) formatQuantData(data *QuantData) string {
	if data == nil {
		return ""
	}

	indicators := e.config.Indicators
	if !indicators.EnableQuantOI && !indicators.EnableQuantNetflow {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 %s Quantitative Data:\n", data.Symbol))

	if len(data.PriceChange) > 0 {
		sb.WriteString("Price Change: ")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}
		parts := []string{}
		for _, tf := range timeframes {
			if v, ok := data.PriceChange[tf]; ok {
				parts = append(parts, fmt.Sprintf("%s: %+.4f%%", tf, v*100))
			}
		}
		sb.WriteString(strings.Join(parts, " | "))
		sb.WriteString("\n")
	}

	if indicators.EnableQuantNetflow && data.Netflow != nil {
		sb.WriteString("Fund Flow (Netflow):\n")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}

		if data.Netflow.Institution != nil {
			if data.Netflow.Institution.Future != nil && len(data.Netflow.Institution.Future) > 0 {
				sb.WriteString("  Institutional Futures:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if data.Netflow.Institution.Spot != nil && len(data.Netflow.Institution.Spot) > 0 {
				sb.WriteString("  Institutional Spot:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}

		if data.Netflow.Personal != nil {
			if data.Netflow.Personal.Future != nil && len(data.Netflow.Personal.Future) > 0 {
				sb.WriteString("  Retail Futures:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if data.Netflow.Personal.Spot != nil && len(data.Netflow.Personal.Spot) > 0 {
				sb.WriteString("  Retail Spot:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}
	}

	if indicators.EnableQuantOI && len(data.OI) > 0 {
		for exchange, oiData := range data.OI {
			if len(oiData.Delta) > 0 {
				sb.WriteString(fmt.Sprintf("Open Interest (%s):\n", exchange))
				for _, tf := range []string{"5m", "15m", "1h", "4h", "12h", "24h"} {
					if d, ok := oiData.Delta[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %+.4f%% (%s)\n", tf, d.OIDeltaPercent, formatFlowValue(d.OIDeltaValue)))
					}
				}
			}
		}
	}

	return sb.String()
}

func formatFlowValue(v float64) string {
	sign := ""
	if v >= 0 {
		sign = "+"
	}
	absV := v
	if absV < 0 {
		absV = -absV
	}
	if absV >= 1e9 {
		return fmt.Sprintf("%s%.2fB", sign, v/1e9)
	} else if absV >= 1e6 {
		return fmt.Sprintf("%s%.2fM", sign, v/1e6)
	} else if absV >= 1e3 {
		return fmt.Sprintf("%s%.2fK", sign, v/1e3)
	}
	return fmt.Sprintf("%s%.2f", sign, v)
}

func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%.4f", v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// ============================================================================
// Indicator Summary (replaces raw arrays with pre-analyzed compact output)
// ============================================================================

func writeIndicatorSummary(sb *strings.Builder, data *market.TimeframeSeriesData, indicators store.IndicatorConfig, currentPrice float64) {
	hasIndicator := false

	if indicators.EnableEMA && len(data.EMA20Values) > 0 {
		hasIndicator = true
		ema20 := sliceLast(data.EMA20Values)
		ema20Dir := sliceTrend(data.EMA20Values)
		if len(data.EMA50Values) > 0 {
			ema50 := sliceLast(data.EMA50Values)
			ema50Dir := sliceTrend(data.EMA50Values)
			relation := "多头排列"
			if ema20 < ema50 {
				relation = "空头排列"
			}
			deviation := 0.0
			if ema50 != 0 {
				deviation = (ema20 - ema50) / ema50 * 100
			}
			sb.WriteString(fmt.Sprintf("EMA: 20=%.2f(%s) vs 50=%.2f(%s) → %s(偏离%+.2f%%)\n",
				ema20, ema20Dir, ema50, ema50Dir, relation, deviation))
		} else {
			sb.WriteString(fmt.Sprintf("EMA20: %.2f(%s)\n", ema20, ema20Dir))
		}
	}

	if indicators.EnableMACD && len(data.MACDValues) > 0 {
		hasIndicator = true
		macdVal := sliceLast(data.MACDValues)
		macdDir := sliceTrend(data.MACDValues)
		momentum := macdMomentumLabel(macdVal, macdDir)
		recent := sliceLastN(data.MACDValues, 3)
		sb.WriteString(fmt.Sprintf("MACD: %.2f(%s) → %s | 近3值: %s\n",
			macdVal, macdDir, momentum, formatCompactFloats(recent)))
	}

	if indicators.EnableRSI {
		parts := []string{}
		if len(data.RSI7Values) > 0 {
			rsi7 := sliceLast(data.RSI7Values)
			recent7 := sliceLastN(data.RSI7Values, 3)
			parts = append(parts, fmt.Sprintf("7期=%.1f(%s) 近3值:%s", rsi7, rsiZoneLabel(rsi7), formatCompactFloats(recent7)))
		}
		if len(data.RSI14Values) > 0 {
			rsi14 := sliceLast(data.RSI14Values)
			recent14 := sliceLastN(data.RSI14Values, 3)
			parts = append(parts, fmt.Sprintf("14期=%.1f(%s) 近3值:%s", rsi14, rsiZoneLabel(rsi14), formatCompactFloats(recent14)))
		}
		if len(parts) > 0 {
			hasIndicator = true
			sb.WriteString(fmt.Sprintf("RSI: %s\n", strings.Join(parts, " | ")))
		}
	}

	if indicators.EnableBOLL && len(data.BOLLUpper) > 0 && len(data.BOLLMiddle) > 0 && len(data.BOLLLower) > 0 {
		hasIndicator = true
		upper := sliceLast(data.BOLLUpper)
		middle := sliceLast(data.BOLLMiddle)
		lower := sliceLast(data.BOLLLower)
		bandwidth := 0.0
		if middle > 0 {
			bandwidth = (upper - lower) / middle * 100
		}
		pos := bollPositionLabel(currentPrice, upper, middle, lower)
		bwTrend := bollBandwidthTrend(data.BOLLUpper, data.BOLLLower, data.BOLLMiddle)
		sb.WriteString(fmt.Sprintf("BOLL(20): 上=%.2f 中=%.2f 下=%.2f → %s | 带宽=%.2f%%%s\n",
			upper, middle, lower, pos, bandwidth, bwTrend))
	}

	if indicators.EnableATR && data.ATR14 > 0 {
		hasIndicator = true
		sb.WriteString(fmt.Sprintf("ATR14: %.4f\n", data.ATR14))
	}

	if hasIndicator {
		sb.WriteString("\n")
	}
}

func sliceLast(s []float64) float64 {
	if len(s) == 0 {
		return 0
	}
	return s[len(s)-1]
}

func sliceTrend(values []float64) string {
	if len(values) < 2 {
		return "→"
	}
	lookback := 5
	if len(values) < lookback+1 {
		lookback = len(values) - 1
	}
	last := values[len(values)-1]
	prev := values[len(values)-1-lookback]
	threshold := math.Abs(prev) * 0.001
	if last-prev > threshold {
		return "↑"
	}
	if prev-last > threshold {
		return "↓"
	}
	return "→"
}

func sliceLastN(s []float64, n int) []float64 {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func macdMomentumLabel(macd float64, dir string) string {
	if macd > 0 {
		if dir == "↑" {
			return "多头动能增强"
		}
		return "多头动能减弱"
	}
	if dir == "↓" {
		return "空头动能增强"
	}
	return "空头动能减弱"
}

func rsiZoneLabel(rsi float64) string {
	switch {
	case rsi >= 80:
		return "极度超买"
	case rsi >= 70:
		return "超买"
	case rsi >= 60:
		return "偏多"
	case rsi >= 40:
		return "中性"
	case rsi >= 30:
		return "偏空"
	case rsi >= 20:
		return "超卖"
	default:
		return "极度超卖"
	}
}

func bollPositionLabel(price, upper, middle, lower float64) string {
	if price == 0 {
		return "N/A"
	}
	if price >= upper {
		return "突破上轨"
	}
	if price <= lower {
		return "跌破下轨"
	}
	bandWidth := upper - lower
	if bandWidth <= 0 {
		return "中轨"
	}
	pos := (price - lower) / bandWidth
	switch {
	case pos >= 0.85:
		return "接近上轨"
	case pos >= 0.6:
		return "中轨上方"
	case pos >= 0.4:
		return "中轨附近"
	case pos >= 0.15:
		return "中轨下方"
	default:
		return "接近下轨"
	}
}

func bollBandwidthTrend(upper, lower, middle []float64) string {
	if len(upper) < 6 || len(lower) < 6 || len(middle) < 6 {
		return ""
	}
	calcBW := func(i int) float64 {
		if middle[i] == 0 {
			return 0
		}
		return (upper[i] - lower[i]) / middle[i]
	}
	current := calcBW(len(upper) - 1)
	prev := calcBW(len(upper) - 6)
	if prev == 0 {
		return ""
	}
	change := (current - prev) / prev
	if change > 0.05 {
		return "(扩张↑)"
	}
	if change < -0.05 {
		return "(收窄↓)"
	}
	return "(平稳→)"
}

func formatCompactFloats(values []float64) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprintf("%.2f", v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
