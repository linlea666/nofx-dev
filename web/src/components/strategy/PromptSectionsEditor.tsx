import { useState } from 'react'
import { ChevronDown, ChevronRight, RotateCcw, FileText, Flame } from 'lucide-react'
import type { PromptSectionsConfig } from '../../types'
import { promptSections as promptSectionsI18n, indicator as indicatorI18n, ts } from '../../i18n/strategy-translations'

interface PromptSectionsEditorProps {
  config: PromptSectionsConfig | undefined
  onChange: (config: PromptSectionsConfig) => void
  disabled?: boolean
  language: string
}

// Default prompt sections (same as backend defaults)
const defaultSections: PromptSectionsConfig = {
  role_definition: `# 你是专业的加密货币交易AI

你专注于技术分析和风险管理，基于市场数据做出理性的交易决策。
你的目标是在控制风险的前提下，捕捉高概率的交易机会。`,

  trading_frequency: `# ⏱️ 交易频率认知

- 优秀交易员：每天2-4笔 ≈ 每小时0.1-0.2笔
- 每小时>2笔 = 过度交易
- 单笔持仓时间≥30-60分钟
如果你发现自己每个周期都在交易 → 标准过低；若持仓<30分钟就平仓 → 过于急躁。`,

  entry_standards: `# 🎯 开仓标准（严格）

只在多重信号共振时开仓：
- 趋势方向明确（EMA排列、价格位置）
- 动量确认（MACD、RSI协同）
- 波动率适中（ATR合理范围）
- 量价配合（成交量支持方向）

避免：单一指标、信号矛盾、横盘震荡、刚平仓即重启。`,

  decision_process: `# 📋 决策流程

1. 检查持仓 → 是否该止盈/止损
2. 扫描候选币 + 多时间框 → 是否存在强信号
3. 评估风险回报比 → 是否满足最小要求
4. 先写思维链，再输出结构化JSON`,
}

const aggressiveSections: PromptSectionsConfig = {
  role_definition: `# 你是一个攻击型加密货币交易AI — 趋势猎手

核心哲学：一小搏大，小亏大赚。你追求的不是高胜率，而是极致的盈亏比。
一笔趋势单的利润必须覆盖5-10次止损的成本。

你的性格特征：
- 对止损毫不犹豫 — 触发即走，零犹豫，止损是成本不是失败
- 对浮盈极度耐心 — 趋势不破绝不主动平仓，浮盈是仓位不是利润
- 方向错了砍仓比呼吸还快，方向对了拿单比石头还稳
- 宁可错过也不做错，但信号来了绝不手软`,

  trading_frequency: `# ⏱️ 交易频率 — 精准出击

- 无趋势/震荡市：保持现金，每天0-2笔，耐心等待
- 趋势确立时：果断进场，允许每天3-6笔（含加仓/滚仓）
- 持仓时间随趋势而定 — 趋势完整可持有数小时甚至跨日
- 核心节奏：快止损（分钟级），慢止盈（小时级）
- 每轮止损后冷静≥3个扫描周期，不报复性交易
- 连续止损3次 → 强制等待60分钟再入场`,

  entry_standards: `# 🎯 入场标准 — 趋势捕获模式

## 模式一：动量突破（置信度≥65）
- EMA20/50金叉或死叉，且斜率加速
- MACD柱状图从零轴翻转，逐根放大
- 放量突破关键阻力/支撑位

## 模式二：趋势回踩（顺势加仓，置信度≥60）
- 4H趋势完好（EMA排列正常）
- 1H级别回踩至EMA20或BOLL中轨附近
- RSI从超买/超卖区回归合理区间
- 回踩时缩量 → 洗盘确认信号

## 模式三：反共识信号（置信度≥70）
- Funding Rate极端（>0.1%或<-0.1%）
- 价格在关键水平但OI持续增加（对手盘拥挤）
- 技术面出现背离（价格新高但RSI未跟）

## ⛔ 止损铁律（不可违反）
- 每笔交易必须设止损 — 无止损 = 无交易
- 止损距离：基于ATR计算，不超过入场价的1.5-2%
- 止损触发 → 立即执行，不移动止损，不心存侥幸`,

  decision_process: `# 📋 决策流程 — 猎手模式

## 第一步：持仓管理（最优先）
盈利持仓 → 执行「持有至失效」规则：
- 浮盈>2% → 止损移至成本价（保本位）
- 浮盈>4% → 止损移至+2%（锁定利润）
- 浮盈>8% → 止损移至+5%（追踪止盈）
- ⚠️ 只有出现【明确反转信号】才主动平仓：
  · 4H级别EMA交叉反转
  · MACD顶/底背离
  · 关键支撑/阻力位被突破
- 没有反转信号 → 默认动作是HOLD，不是平仓！

亏损持仓 → 止损价到了就砍，不犹豫，不移动止损

## 第二步：扫描市场状态
- 趋势市 → 激活趋势追踪，寻找入场/加仓机会
- 震荡市 → 仅观望或做区间边缘反转（极低频）
- 高波动 → 缩小仓位，放宽止损

## 第三步：精确入场
4H定方向 → 1H定结构 → 15M/5M精确入场
满足入场模式一/二/三中任一即可

## 第四步：仓位计算
根据止损距离反算仓位，单笔风险不超过账户权益的2-3%

## 第五步：输出
先写思维链（简洁有力），再输出结构化JSON`,
}

export function PromptSectionsEditor({
  config,
  onChange,
  disabled,
  language,
}: PromptSectionsEditorProps) {
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({
    role_definition: false,
    trading_frequency: false,
    entry_standards: false,
    decision_process: false,
  })

  const sections = [
    { key: 'role_definition', label: ts(promptSectionsI18n.roleDefinition, language), desc: ts(promptSectionsI18n.roleDefinitionDesc, language) },
    { key: 'trading_frequency', label: ts(promptSectionsI18n.tradingFrequency, language), desc: ts(promptSectionsI18n.tradingFrequencyDesc, language) },
    { key: 'entry_standards', label: ts(promptSectionsI18n.entryStandards, language), desc: ts(promptSectionsI18n.entryStandardsDesc, language) },
    { key: 'decision_process', label: ts(promptSectionsI18n.decisionProcess, language), desc: ts(promptSectionsI18n.decisionProcessDesc, language) },
  ]

  const currentConfig = config || {}

  const updateSection = (key: keyof PromptSectionsConfig, value: string) => {
    if (!disabled) {
      onChange({ ...currentConfig, [key]: value })
    }
  }

  const resetSection = (key: keyof PromptSectionsConfig) => {
    if (!disabled) {
      onChange({ ...currentConfig, [key]: defaultSections[key] })
    }
  }

  const applyAggressivePreset = () => {
    if (!disabled) {
      onChange({ ...aggressiveSections })
    }
  }

  const toggleSection = (key: string) => {
    setExpandedSections((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  const getValue = (key: keyof PromptSectionsConfig): string => {
    return currentConfig[key] || defaultSections[key] || ''
  }

  return (
    <div className="space-y-4">
      <div className="flex items-start gap-2 mb-4">
        <FileText className="w-5 h-5 mt-0.5" style={{ color: '#a855f7' }} />
        <div>
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {ts(promptSectionsI18n.promptSections, language)}
          </h3>
          <p className="text-xs mt-1" style={{ color: '#848E9C' }}>
            {ts(promptSectionsI18n.promptSectionsDesc, language)}
          </p>
        </div>
      </div>

      {/* Aggressive Preset Button */}
      <div className="flex items-center justify-between px-1">
        <span className="text-xs" style={{ color: '#848E9C' }}>
          {ts(indicatorI18n.aggressivePresetDesc, language)}
        </span>
        <button
          onClick={applyAggressivePreset}
          disabled={disabled}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all hover:scale-[1.02]"
          style={{
            background: 'linear-gradient(135deg, rgba(234, 57, 67, 0.18) 0%, rgba(255, 140, 0, 0.10) 100%)',
            border: '1px solid rgba(234, 57, 67, 0.45)',
            color: '#F6465D',
          }}
        >
          <Flame className="w-3.5 h-3.5" />
          {ts(indicatorI18n.aggressivePreset, language)}
        </button>
      </div>

      <div className="space-y-2">
        {sections.map(({ key, label, desc }) => {
          const sectionKey = key as keyof PromptSectionsConfig
          const isExpanded = expandedSections[key]
          const value = getValue(sectionKey)
          const isModified = currentConfig[sectionKey] !== undefined && currentConfig[sectionKey] !== defaultSections[sectionKey]

          return (
            <div
              key={key}
              className="rounded-lg overflow-hidden"
              style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
            >
              <button
                onClick={() => toggleSection(key)}
                className="w-full flex items-center justify-between px-3 py-2.5 hover:bg-white/5 transition-colors text-left"
              >
                <div className="flex items-center gap-2">
                  {isExpanded ? (
                    <ChevronDown className="w-4 h-4" style={{ color: '#848E9C' }} />
                  ) : (
                    <ChevronRight className="w-4 h-4" style={{ color: '#848E9C' }} />
                  )}
                  <span className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                    {label}
                  </span>
                  {isModified && (
                    <span
                      className="px-1.5 py-0.5 text-[10px] rounded"
                      style={{ background: 'rgba(168, 85, 247, 0.15)', color: '#a855f7' }}
                    >
                      {ts(promptSectionsI18n.modified, language)}
                    </span>
                  )}
                </div>
                <span className="text-[10px]" style={{ color: '#848E9C' }}>
                  {value.length} {ts(promptSectionsI18n.chars, language)}
                </span>
              </button>

              {isExpanded && (
                <div className="px-3 pb-3">
                  <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                    {desc}
                  </p>
                  <textarea
                    value={value}
                    onChange={(e) => updateSection(sectionKey, e.target.value)}
                    disabled={disabled}
                    rows={6}
                    className="w-full px-3 py-2 rounded-lg resize-y font-mono text-xs"
                    style={{
                      background: '#1E2329',
                      border: '1px solid #2B3139',
                      color: '#EAECEF',
                      minHeight: '120px',
                    }}
                  />
                  <div className="flex justify-end mt-2">
                    <button
                      onClick={() => resetSection(sectionKey)}
                      disabled={disabled || !isModified}
                      className="flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors hover:bg-white/5 disabled:opacity-30"
                      style={{ color: '#848E9C' }}
                    >
                      <RotateCcw className="w-3 h-3" />
                      {ts(promptSectionsI18n.resetToDefault, language)}
                    </button>
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
