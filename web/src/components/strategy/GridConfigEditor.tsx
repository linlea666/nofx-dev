import { useState } from 'react'
import { Grid, DollarSign, TrendingUp, Shield, Compass, Activity } from 'lucide-react'
import type { GridStrategyConfig } from '../../types'
import { gridConfig, ts } from '../../i18n/strategy-translations'

interface GridConfigEditorProps {
  config: GridStrategyConfig
  onChange: (config: GridStrategyConfig) => void
  disabled?: boolean
  language: string
}

// Volatility presets for different asset classes
const PRESETS = {
  crypto: {
    narrow_bb_width: 2.0, standard_bb_width: 3.0, wide_bb_width: 4.0,
    narrow_atr_pct: 1.0, standard_atr_pct: 2.0, wide_atr_pct: 3.0,
    ranging_bb_width: 3.0, trending_bb_width: 4.0,
    ranging_ema_dist: 1.0, trending_ema_dist: 2.0,
    breakout_pause_pct: 2.0,
    atr_multiplier: 2.0, grid_count: 10, leverage: 5,
    max_drawdown_pct: 15, stop_loss_pct: 5, daily_loss_limit_pct: 10,
  },
  gold: {
    narrow_bb_width: 0.8, standard_bb_width: 1.5, wide_bb_width: 2.5,
    narrow_atr_pct: 0.3, standard_atr_pct: 0.7, wide_atr_pct: 1.2,
    ranging_bb_width: 1.5, trending_bb_width: 2.5,
    ranging_ema_dist: 1.0, trending_ema_dist: 2.0,
    breakout_pause_pct: 1.0,
    atr_multiplier: 3.5, grid_count: 15, leverage: 3,
    max_drawdown_pct: 8, stop_loss_pct: 2, daily_loss_limit_pct: 5,
    bounds_mode: 'box' as string, box_bounds_period: 'mid' as string,
  },
} as const

type PresetKey = 'crypto' | 'gold' | 'custom'

const LOW_VOL_SYMBOLS = ['GOLD', 'SILVER', 'EUR', 'JPY']

function detectPreset(symbol: string): PresetKey {
  const base = symbol.replace(/USDT$/, '').toUpperCase()
  if (LOW_VOL_SYMBOLS.includes(base)) return 'gold'
  return 'crypto'
}

// Default grid configuration (spread first, explicit overrides after)
export const defaultGridConfig: GridStrategyConfig = {
  ...PRESETS.crypto,
  symbol: 'BTCUSDT',
  total_investment: 1000,
  upper_price: 0,
  lower_price: 0,
  use_atr_bounds: true,
  distribution: 'gaussian',
  use_maker_only: true,
  enable_direction_adjust: false,
  direction_bias_ratio: 0.7,
}

export function GridConfigEditor({
  config,
  onChange,
  disabled,
  language,
}: GridConfigEditorProps) {
  const [activePreset, setActivePreset] = useState<PresetKey>(() => detectPreset(config.symbol))
  const [showVolProfile, setShowVolProfile] = useState(false)

  const updateField = <K extends keyof GridStrategyConfig>(
    key: K,
    value: GridStrategyConfig[K]
  ) => {
    if (!disabled) {
      onChange({ ...config, [key]: value })
    }
  }

  const applyPreset = (preset: PresetKey) => {
    setActivePreset(preset)
    if (preset !== 'custom' && !disabled) {
      const p = PRESETS[preset]
      onChange({ ...config, ...p })
    }
  }

  const handleSymbolChange = (symbol: string) => {
    if (disabled) return
    const newPreset = detectPreset(symbol)
    const oldPreset = detectPreset(config.symbol)
    if (newPreset !== oldPreset && newPreset !== 'custom') {
      const p = PRESETS[newPreset]
      setActivePreset(newPreset)
      onChange({ ...config, symbol, ...p })
    } else {
      onChange({ ...config, symbol })
    }
  }

  const inputStyle = {
    background: '#1E2329',
    border: '1px solid #2B3139',
    color: '#EAECEF',
  }

  const sectionStyle = {
    background: '#0B0E11',
    border: '1px solid #2B3139',
  }

  return (
    <div className="space-y-6">
      {/* Trading Setup */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <DollarSign className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {ts(gridConfig.tradingPair, language)}
          </h3>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {/* Symbol */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(gridConfig.symbol, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(gridConfig.symbolDesc, language)}
            </p>
            <select
              value={config.symbol}
              onChange={(e) => handleSymbolChange(e.target.value)}
              disabled={disabled}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            >
              <optgroup label={ts(gridConfig.symbolGroupCrypto, language)}>
                <option value="BTCUSDT">BTC/USDT</option>
                <option value="ETHUSDT">ETH/USDT</option>
                <option value="SOLUSDT">SOL/USDT</option>
                <option value="BNBUSDT">BNB/USDT</option>
                <option value="XRPUSDT">XRP/USDT</option>
                <option value="DOGEUSDT">DOGE/USDT</option>
              </optgroup>
              <optgroup label={ts(gridConfig.symbolGroupHyperliquid, language)}>
                <option value="GOLD">GOLD (黄金)</option>
                <option value="SILVER">SILVER (白银)</option>
                <option value="EUR">EUR (欧元)</option>
                <option value="JPY">JPY (日元)</option>
              </optgroup>
            </select>
          </div>

          {/* Investment */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(gridConfig.totalInvestment, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(gridConfig.totalInvestmentDesc, language)}
            </p>
            <input
              type="number"
              value={config.total_investment}
              onChange={(e) => updateField('total_investment', parseFloat(e.target.value) || 1000)}
              disabled={disabled}
              min={100}
              step={100}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>

          {/* Leverage */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(gridConfig.leverage, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(gridConfig.leverageDesc, language)}
            </p>
            <input
              type="number"
              value={config.leverage}
              onChange={(e) => updateField('leverage', parseInt(e.target.value) || 5)}
              disabled={disabled}
              min={1}
              max={5}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>
        </div>
      </div>

      {/* Grid Parameters */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Grid className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {ts(gridConfig.gridParameters, language)}
          </h3>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Grid Count */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(gridConfig.gridCount, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(gridConfig.gridCountDesc, language)}
            </p>
            <input
              type="number"
              value={config.grid_count}
              onChange={(e) => updateField('grid_count', parseInt(e.target.value) || 10)}
              disabled={disabled}
              min={5}
              max={50}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>

          {/* Distribution */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(gridConfig.distribution, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(gridConfig.distributionDesc, language)}
            </p>
            <select
              value={config.distribution}
              onChange={(e) => updateField('distribution', e.target.value as 'uniform' | 'gaussian' | 'pyramid')}
              disabled={disabled}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            >
              <option value="uniform">{ts(gridConfig.uniform, language)}</option>
              <option value="gaussian">{ts(gridConfig.gaussian, language)}</option>
              <option value="pyramid">{ts(gridConfig.pyramid, language)}</option>
            </select>
          </div>
        </div>
      </div>

      {/* Price Bounds */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <TrendingUp className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {ts(gridConfig.priceBounds, language)}
          </h3>
        </div>

        {/* Bounds Mode Selector */}
        <div className="p-4 rounded-lg mb-4" style={sectionStyle}>
          <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
            {language === 'zh' ? '边界计算方式' : 'Bounds Calculation'}
          </label>
          <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
            {language === 'zh' ? '选择网格上下边界的计算方式' : 'Choose how grid boundaries are calculated'}
          </p>
          <div className="flex gap-2">
            {[
              { value: 'atr', label: { zh: 'ATR 自动', en: 'ATR Auto' } },
              { value: 'box', label: { zh: '箱体 (Donchian)', en: 'Box (Donchian)' } },
              { value: 'manual', label: { zh: '手动设定', en: 'Manual' } },
            ].map(opt => (
              <button
                key={opt.value}
                onClick={() => {
                  updateField('bounds_mode', opt.value)
                  updateField('use_atr_bounds', opt.value === 'atr')
                }}
                disabled={disabled}
                className="px-3 py-1.5 rounded text-sm transition-colors"
                style={{
                  background: (config.bounds_mode || (config.use_atr_bounds ? 'atr' : 'manual')) === opt.value ? '#F0B90B' : '#2B3139',
                  color: (config.bounds_mode || (config.use_atr_bounds ? 'atr' : 'manual')) === opt.value ? '#0B0E11' : '#848E9C',
                  border: '1px solid #2B3139',
                }}
              >
                {ts(opt.label, language)}
              </button>
            ))}
          </div>
        </div>

        {/* Mode-specific options */}
        {(config.bounds_mode || (config.use_atr_bounds ? 'atr' : 'manual')) === 'atr' && (
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(gridConfig.atrMultiplier, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(gridConfig.atrMultiplierDesc, language)}
            </p>
            <input
              type="number"
              value={config.atr_multiplier}
              onChange={(e) => updateField('atr_multiplier', parseFloat(e.target.value) || 2.0)}
              disabled={disabled}
              min={1}
              max={5}
              step={0.5}
              className="w-32 px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>
        )}

        {(config.bounds_mode || (config.use_atr_bounds ? 'atr' : 'manual')) === 'box' && (
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {language === 'zh' ? '箱体周期' : 'Box Period'}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {language === 'zh' ? '使用哪个时间周期的 Donchian 通道作为网格边界' : 'Which Donchian channel period to use as grid boundaries'}
            </p>
            <div className="flex gap-2">
              {[
                { value: 'short', label: { zh: '短期 (3天)', en: 'Short (3d)' } },
                { value: 'mid', label: { zh: '中期 (10天)', en: 'Mid (10d)' } },
                { value: 'long', label: { zh: '长期 (21天)', en: 'Long (21d)' } },
              ].map(opt => (
                <button
                  key={opt.value}
                  onClick={() => updateField('box_bounds_period', opt.value)}
                  disabled={disabled}
                  className="px-3 py-1.5 rounded text-sm transition-colors"
                  style={{
                    background: (config.box_bounds_period || 'mid') === opt.value ? '#F0B90B' : '#2B3139',
                    color: (config.box_bounds_period || 'mid') === opt.value ? '#0B0E11' : '#848E9C',
                    border: '1px solid #2B3139',
                  }}
                >
                  {ts(opt.label, language)}
                </button>
              ))}
            </div>
          </div>
        )}

        {(config.bounds_mode || (config.use_atr_bounds ? 'atr' : 'manual')) === 'manual' && (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                {ts(gridConfig.upperPrice, language)}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {ts(gridConfig.upperPriceDesc, language)}
              </p>
              <input
                type="number"
                value={config.upper_price}
                onChange={(e) => updateField('upper_price', parseFloat(e.target.value) || 0)}
                disabled={disabled}
                min={0}
                step={0.01}
                className="w-full px-3 py-2 rounded"
                style={inputStyle}
              />
            </div>
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                {ts(gridConfig.lowerPrice, language)}
              </label>
              <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
                {ts(gridConfig.lowerPriceDesc, language)}
              </p>
              <input
                type="number"
                value={config.lower_price}
                onChange={(e) => updateField('lower_price', parseFloat(e.target.value) || 0)}
                disabled={disabled}
                min={0}
                step={0.01}
                className="w-full px-3 py-2 rounded"
                style={inputStyle}
              />
            </div>
          </div>
        )}
      </div>

      {/* Risk Control */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {ts(gridConfig.riskControl, language)}
          </h3>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(gridConfig.maxDrawdown, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(gridConfig.maxDrawdownDesc, language)}
            </p>
            <input
              type="number"
              value={config.max_drawdown_pct}
              onChange={(e) => updateField('max_drawdown_pct', parseFloat(e.target.value) || 15)}
              disabled={disabled}
              min={5}
              max={50}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>

          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(gridConfig.stopLoss, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(gridConfig.stopLossDesc, language)}
            </p>
            <input
              type="number"
              value={config.stop_loss_pct}
              onChange={(e) => updateField('stop_loss_pct', parseFloat(e.target.value) || 5)}
              disabled={disabled}
              min={1}
              max={20}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>

          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(gridConfig.dailyLossLimit, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(gridConfig.dailyLossLimitDesc, language)}
            </p>
            <input
              type="number"
              value={config.daily_loss_limit_pct}
              onChange={(e) => updateField('daily_loss_limit_pct', parseFloat(e.target.value) || 10)}
              disabled={disabled}
              min={1}
              max={30}
              className="w-full px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Maker Only Toggle */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <div className="flex items-center justify-between">
              <div>
                <label className="block text-sm" style={{ color: '#EAECEF' }}>
                  {ts(gridConfig.useMakerOnly, language)}
                </label>
                <p className="text-xs" style={{ color: '#848E9C' }}>
                  {ts(gridConfig.useMakerOnlyDesc, language)}
                </p>
              </div>
              <label className="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  checked={config.use_maker_only}
                  onChange={(e) => updateField('use_maker_only', e.target.checked)}
                  disabled={disabled}
                  className="sr-only peer"
                />
                <div className="w-11 h-6 bg-gray-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-[#F0B90B]"></div>
              </label>
            </div>
          </div>

          {/* Breakout Pause Threshold */}
          <div className="p-4 rounded-lg" style={sectionStyle}>
            <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
              {ts(gridConfig.breakoutPausePct, language)}
            </label>
            <p className="text-xs mb-2" style={{ color: '#848E9C' }}>
              {ts(gridConfig.breakoutPausePctDesc, language)}
            </p>
            <input
              type="number"
              value={config.breakout_pause_pct ?? 2.0}
              onChange={(e) => updateField('breakout_pause_pct', parseFloat(e.target.value) || 2.0)}
              disabled={disabled}
              min={0.5}
              max={10}
              step={0.5}
              className="w-32 px-3 py-2 rounded"
              style={inputStyle}
            />
          </div>
        </div>
      </div>

      {/* Direction Auto-Adjust */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Compass className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {ts(gridConfig.directionAdjust, language)}
          </h3>
        </div>

        {/* Enable Toggle */}
        <div className="p-4 rounded-lg mb-4" style={sectionStyle}>
          <div className="flex items-center justify-between">
            <div>
              <label className="block text-sm" style={{ color: '#EAECEF' }}>
                {ts(gridConfig.enableDirectionAdjust, language)}
              </label>
              <p className="text-xs" style={{ color: '#848E9C' }}>
                {ts(gridConfig.enableDirectionAdjustDesc, language)}
              </p>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={config.enable_direction_adjust ?? false}
                onChange={(e) => updateField('enable_direction_adjust', e.target.checked)}
                disabled={disabled}
                className="sr-only peer"
              />
              <div className="w-11 h-6 bg-gray-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-[#F0B90B]"></div>
            </label>
          </div>
        </div>

        {config.enable_direction_adjust && (
          <>
            {/* Direction Modes Explanation */}
            <div className="p-4 rounded-lg mb-4" style={{ background: '#1E2329', border: '1px solid #F0B90B33' }}>
              <p className="text-xs font-medium mb-2" style={{ color: '#F0B90B' }}>
                📊 {ts(gridConfig.directionModes, language)}
              </p>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-2 text-xs" style={{ color: '#848E9C' }}>
                <div>• {ts(gridConfig.modeNeutral, language)}</div>
                <div>• <span style={{ color: '#0ECB81' }}>{ts(gridConfig.modeLongBias, language)}</span></div>
                <div>• <span style={{ color: '#0ECB81' }}>{ts(gridConfig.modeLong, language)}</span></div>
                <div>• <span style={{ color: '#F6465D' }}>{ts(gridConfig.modeShortBias, language)}</span></div>
                <div>• <span style={{ color: '#F6465D' }}>{ts(gridConfig.modeShort, language)}</span></div>
              </div>
              <p className="text-xs mt-3 pt-2 border-t border-zinc-700" style={{ color: '#848E9C' }}>
                💡 {ts(gridConfig.directionExplain, language)}
              </p>
            </div>

            {/* Bias Strength */}
            <div className="p-4 rounded-lg" style={sectionStyle}>
              <label className="block text-sm mb-1" style={{ color: '#EAECEF' }}>
                {ts(gridConfig.directionBiasRatio, language)} (X)
              </label>
              <p className="text-xs mb-1" style={{ color: '#848E9C' }}>
                {ts(gridConfig.directionBiasRatioDesc, language)}
              </p>
              <p className="text-xs mb-3" style={{ color: '#F0B90B' }}>
                {ts(gridConfig.directionBiasExplain, language)}
              </p>
              <div className="flex items-center gap-3">
                <input
                  type="range"
                  value={(config.direction_bias_ratio ?? 0.7) * 100}
                  onChange={(e) => updateField('direction_bias_ratio', parseInt(e.target.value) / 100)}
                  disabled={disabled}
                  min={55}
                  max={90}
                  step={5}
                  className="flex-1 h-2 rounded-lg appearance-none cursor-pointer"
                  style={{ background: '#2B3139' }}
                />
                <span className="text-sm font-mono w-20 text-right" style={{ color: '#F0B90B' }}>
                  X = {Math.round((config.direction_bias_ratio ?? 0.7) * 100)}%
                </span>
              </div>
              <div className="mt-2 grid grid-cols-2 gap-2 text-xs">
                <div className="p-2 rounded" style={{ background: '#0ECB8115', border: '1px solid #0ECB8130' }}>
                  <span style={{ color: '#0ECB81' }}>Long Bias: </span>
                  <span style={{ color: '#EAECEF' }}>{Math.round((config.direction_bias_ratio ?? 0.7) * 100)}% {ts(gridConfig.buy, language)} + {Math.round((1 - (config.direction_bias_ratio ?? 0.7)) * 100)}% {ts(gridConfig.sell, language)}</span>
                </div>
                <div className="p-2 rounded" style={{ background: '#F6465D15', border: '1px solid #F6465D30' }}>
                  <span style={{ color: '#F6465D' }}>Short Bias: </span>
                  <span style={{ color: '#EAECEF' }}>{Math.round((1 - (config.direction_bias_ratio ?? 0.7)) * 100)}% {ts(gridConfig.buy, language)} + {Math.round((config.direction_bias_ratio ?? 0.7) * 100)}% {ts(gridConfig.sell, language)}</span>
                </div>
              </div>
            </div>
          </>
        )}
      </div>

      {/* Volatility Profile */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Activity className="w-5 h-5" style={{ color: '#F0B90B' }} />
          <h3 className="font-medium" style={{ color: '#EAECEF' }}>
            {ts(gridConfig.volatilityProfile, language)}
          </h3>
        </div>

        <p className="text-xs mb-4" style={{ color: '#848E9C' }}>
          {ts(gridConfig.volatilityProfileDesc, language)}
        </p>

        {/* Preset Selector */}
        <div className="flex gap-2 mb-4">
          {(['crypto', 'gold', 'custom'] as const).map((key) => {
            const labelKey = key === 'crypto' ? 'presetCrypto' : key === 'gold' ? 'presetGold' : 'presetCustom'
            return (
              <button
                key={key}
                onClick={() => applyPreset(key)}
                disabled={disabled}
                className="px-4 py-2 rounded text-sm transition-colors"
                style={{
                  background: activePreset === key ? '#F0B90B' : '#1E2329',
                  color: activePreset === key ? '#0B0E11' : '#848E9C',
                  border: `1px solid ${activePreset === key ? '#F0B90B' : '#2B3139'}`,
                  fontWeight: activePreset === key ? 600 : 400,
                }}
              >
                {ts(gridConfig[labelKey], language)}
              </button>
            )
          })}
        </div>

        {/* Expandable Detail */}
        <button
          onClick={() => setShowVolProfile(!showVolProfile)}
          className="text-xs mb-3 cursor-pointer"
          style={{ color: '#F0B90B' }}
        >
          {showVolProfile ? '▼' : '▶'} {language === 'zh' ? '展开详细阈值配置' : 'Expand detailed threshold config'}
        </button>

        {showVolProfile && (
          <div className="space-y-4">
            {/* Regime Classification Thresholds */}
            <div className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
              <p className="text-xs font-medium mb-3" style={{ color: '#EAECEF' }}>
                {language === 'zh' ? 'Regime 分类阈值 (布林带宽度 % / ATR %)' : 'Regime Classification (BB Width % / ATR %)'}
              </p>
              <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
                {([
                  ['narrow_bb_width', gridConfig.narrowBBWidth],
                  ['standard_bb_width', gridConfig.standardBBWidth],
                  ['wide_bb_width', gridConfig.wideBBWidth],
                  ['narrow_atr_pct', gridConfig.narrowATRPct],
                  ['standard_atr_pct', gridConfig.standardATRPct],
                  ['wide_atr_pct', gridConfig.wideATRPct],
                ] as [keyof GridStrategyConfig, { zh: string; en: string }][]).map(([field, label]) => (
                  <div key={field}>
                    <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                      {ts(label, language)}
                    </label>
                    <input
                      type="number"
                      value={(config[field] as number) ?? 0}
                      onChange={(e) => updateField(field, parseFloat(e.target.value) || 0)}
                      disabled={disabled}
                      min={0}
                      step={0.1}
                      className="w-full px-2 py-1 rounded text-sm"
                      style={inputStyle}
                    />
                  </div>
                ))}
              </div>
            </div>

            {/* AI Prompt Thresholds */}
            <div className="p-4 rounded-lg" style={{ background: '#0B0E11', border: '1px solid #2B3139' }}>
              <p className="text-xs font-medium mb-3" style={{ color: '#EAECEF' }}>
                {language === 'zh' ? 'AI Prompt 市场判定阈值' : 'AI Prompt Market Regime Thresholds'}
              </p>
              <div className="grid grid-cols-2 gap-3">
                {([
                  ['ranging_bb_width', gridConfig.rangingBBWidth],
                  ['trending_bb_width', gridConfig.trendingBBWidth],
                  ['ranging_ema_dist', gridConfig.rangingEMADist],
                  ['trending_ema_dist', gridConfig.trendingEMADist],
                ] as [keyof GridStrategyConfig, { zh: string; en: string }][]).map(([field, label]) => (
                  <div key={field}>
                    <label className="block text-xs mb-1" style={{ color: '#848E9C' }}>
                      {ts(label, language)}
                    </label>
                    <input
                      type="number"
                      value={(config[field] as number) ?? 0}
                      onChange={(e) => updateField(field, parseFloat(e.target.value) || 0)}
                      disabled={disabled}
                      min={0}
                      step={0.1}
                      className="w-full px-2 py-1 rounded text-sm"
                      style={inputStyle}
                    />
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
