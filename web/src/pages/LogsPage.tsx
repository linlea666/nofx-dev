import { useState, useEffect, useRef, useCallback } from 'react'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { logApi, type LogEntry } from '../lib/api/logs'
import { DeepVoidBackground } from '../components/common/DeepVoidBackground'
import { Download, RefreshCw, ArrowDown, Pause, Play } from 'lucide-react'

const CATEGORIES = [
  { key: '', label: { zh: '全部', en: 'All' } },
  { key: 'grid', label: { zh: '网格', en: 'Grid' } },
  { key: 'exchange', label: { zh: '交易所', en: 'Exchange' } },
  { key: 'ai', label: { zh: 'AI', en: 'AI' } },
  { key: 'system', label: { zh: '系统', en: 'System' } },
]

const LEVELS = [
  { key: '', label: 'ALL' },
  { key: 'info', label: 'INFO' },
  { key: 'warn', label: 'WARN' },
  { key: 'error', label: 'ERROR' },
]

const LEVEL_COLORS: Record<string, string> = {
  INFO: '#7FE7CC',
  WARNING: '#F0B90B',
  WARN: '#F0B90B',
  ERROR: '#F6465D',
  ERRO: '#F6465D',
  DEBUG: '#848E9C',
  FATAL: '#F6465D',
  PANIC: '#F6465D',
}

const CATEGORY_COLORS: Record<string, string> = {
  grid: '#F0B90B',
  exchange: '#7FE7CC',
  ai: '#3B82F6',
  system: '#848E9C',
}

const LINE_LIMITS = [100, 200, 500]

export default function LogsPage() {
  const { language } = useLanguage()
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [latestId, setLatestId] = useState(0)
  const [category, setCategory] = useState('')
  const [level, setLevel] = useState('')
  const [lineLimit, setLineLimit] = useState(200)
  const [autoScroll, setAutoScroll] = useState(true)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [showExport, setShowExport] = useState(false)
  const [exportDates, setExportDates] = useState<string[]>([])
  const logContainerRef = useRef<HTMLDivElement>(null)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const fetchLogs = useCallback(async (incremental: boolean) => {
    try {
      const sinceId = incremental ? latestId : 0
      const data = await logApi.getLogs(sinceId, lineLimit, level, category)

      if (incremental && sinceId > 0) {
        setEntries(prev => {
          const combined = [...prev, ...data.entries]
          if (combined.length > lineLimit) {
            return combined.slice(combined.length - lineLimit)
          }
          return combined
        })
      } else {
        setEntries(data.entries)
      }

      if (data.latest_id > 0) {
        setLatestId(data.latest_id)
      }
    } catch {
      // silently fail on fetch errors
    }
  }, [latestId, lineLimit, level, category])

  // Initial load + full reload on filter change
  useEffect(() => {
    setLatestId(0)
    setEntries([])
    const load = async () => {
      try {
        const data = await logApi.getLogs(0, lineLimit, level, category)
        setEntries(data.entries)
        if (data.latest_id > 0) setLatestId(data.latest_id)
      } catch { /* ignore */ }
    }
    load()
  }, [category, level, lineLimit])

  // Auto-refresh polling
  useEffect(() => {
    if (intervalRef.current) clearInterval(intervalRef.current)
    if (autoRefresh) {
      intervalRef.current = setInterval(() => fetchLogs(true), 2000)
    }
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current)
    }
  }, [autoRefresh, fetchLogs])

  // Auto-scroll to bottom
  useEffect(() => {
    if (autoScroll && logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight
    }
  }, [entries, autoScroll])

  // Load export dates on mount
  useEffect(() => {
    logApi.getLogDates().then(setExportDates).catch(() => {})
  }, [])

  const handleExport = (format: 'csv' | 'json', date?: string) => {
    const url = logApi.getExportUrl(format, category || undefined, date)
    window.open(url, '_blank')
    setShowExport(false)
  }

  const zh = language === 'zh'

  return (
    <DeepVoidBackground className="py-8" disableAnimation>
      <div className="w-full px-4 md:px-8 space-y-4 animate-fade-in">
        {/* Header */}
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {t('logsPageTitle', language)}
          </h1>

          {/* Export dropdown */}
          <div className="relative">
            <button
              onClick={() => setShowExport(!showExport)}
              className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold transition-all hover:scale-105"
              style={{ background: '#F0B90B20', border: '1px solid #F0B90B', color: '#F0B90B' }}
            >
              <Download className="w-4 h-4" />
              {zh ? '导出日志' : 'Export'}
            </button>

            {showExport && (
              <div
                className="absolute right-0 top-full mt-2 w-64 rounded-xl shadow-2xl z-50 p-3 space-y-2"
                style={{ background: '#1E2329', border: '1px solid #2B3139' }}
              >
                <div className="text-xs font-semibold mb-2" style={{ color: '#848E9C' }}>
                  {zh ? '选择日期和格式' : 'Select date & format'}
                </div>

                {exportDates.length === 0 && (
                  <div className="text-xs" style={{ color: '#848E9C' }}>
                    {zh ? '暂无日志文件' : 'No log files'}
                  </div>
                )}

                {exportDates.map(date => (
                  <div key={date} className="flex items-center justify-between gap-2">
                    <span className="text-xs font-mono" style={{ color: '#EAECEF' }}>{date}</span>
                    <div className="flex gap-1">
                      <button
                        onClick={() => handleExport('csv', date)}
                        className="px-2 py-1 rounded text-xs font-semibold transition-all hover:scale-105"
                        style={{ background: '#7FE7CC20', color: '#7FE7CC' }}
                      >
                        CSV
                      </button>
                      <button
                        onClick={() => handleExport('json', date)}
                        className="px-2 py-1 rounded text-xs font-semibold transition-all hover:scale-105"
                        style={{ background: '#3B82F620', color: '#3B82F6' }}
                      >
                        JSON
                      </button>
                    </div>
                  </div>
                ))}

                <div
                  className="border-t pt-2 mt-2"
                  style={{ borderColor: '#2B3139' }}
                >
                  <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                    {zh ? `当前筛选: ${category || '全部'}` : `Filter: ${category || 'all'}`}
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Toolbar */}
        <div className="flex flex-wrap items-center gap-3 p-3 rounded-xl" style={{ background: '#1E232930', border: '1px solid #2B3139' }}>
          {/* Category tabs */}
          <div className="flex gap-1">
            {CATEGORIES.map(cat => (
              <button
                key={cat.key}
                onClick={() => setCategory(cat.key)}
                className="px-3 py-1.5 rounded-lg text-xs font-semibold transition-all"
                style={{
                  background: category === cat.key
                    ? (cat.key ? `${CATEGORY_COLORS[cat.key]}20` : '#F0B90B20')
                    : 'transparent',
                  border: `1px solid ${category === cat.key
                    ? (cat.key ? CATEGORY_COLORS[cat.key] : '#F0B90B')
                    : '#2B3139'}`,
                  color: category === cat.key
                    ? (cat.key ? CATEGORY_COLORS[cat.key] : '#F0B90B')
                    : '#848E9C',
                }}
              >
                {zh ? cat.label.zh : cat.label.en}
              </button>
            ))}
          </div>

          <div className="w-px h-6" style={{ background: '#2B3139' }} />

          {/* Level filter */}
          <div className="flex gap-1">
            {LEVELS.map(lv => (
              <button
                key={lv.key}
                onClick={() => setLevel(lv.key)}
                className="px-2.5 py-1.5 rounded-lg text-xs font-semibold transition-all"
                style={{
                  background: level === lv.key ? '#EAECEF15' : 'transparent',
                  border: `1px solid ${level === lv.key ? '#EAECEF40' : '#2B3139'}`,
                  color: level === lv.key
                    ? (lv.key ? LEVEL_COLORS[lv.label] || '#EAECEF' : '#EAECEF')
                    : '#848E9C',
                }}
              >
                {lv.label}
              </button>
            ))}
          </div>

          <div className="flex-1" />

          {/* Line limit */}
          <select
            value={lineLimit}
            onChange={e => setLineLimit(Number(e.target.value))}
            className="px-2 py-1.5 rounded-lg text-xs font-mono"
            style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
          >
            {LINE_LIMITS.map(n => (
              <option key={n} value={n}>{n} {zh ? '行' : 'lines'}</option>
            ))}
          </select>

          {/* Auto-scroll toggle */}
          <button
            onClick={() => setAutoScroll(!autoScroll)}
            className="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-semibold transition-all"
            style={{
              background: autoScroll ? '#7FE7CC15' : 'transparent',
              border: `1px solid ${autoScroll ? '#7FE7CC' : '#2B3139'}`,
              color: autoScroll ? '#7FE7CC' : '#848E9C',
            }}
            title={zh ? '自动滚动' : 'Auto-scroll'}
          >
            <ArrowDown className="w-3.5 h-3.5" />
          </button>

          {/* Auto-refresh toggle */}
          <button
            onClick={() => setAutoRefresh(!autoRefresh)}
            className="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-semibold transition-all"
            style={{
              background: autoRefresh ? '#7FE7CC15' : 'transparent',
              border: `1px solid ${autoRefresh ? '#7FE7CC' : '#2B3139'}`,
              color: autoRefresh ? '#7FE7CC' : '#848E9C',
            }}
            title={zh ? (autoRefresh ? '暂停刷新' : '恢复刷新') : (autoRefresh ? 'Pause' : 'Resume')}
          >
            {autoRefresh ? <Pause className="w-3.5 h-3.5" /> : <Play className="w-3.5 h-3.5" />}
          </button>

          {/* Manual refresh */}
          <button
            onClick={() => fetchLogs(false)}
            className="flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-semibold transition-all hover:scale-105"
            style={{ background: 'transparent', border: '1px solid #2B3139', color: '#848E9C' }}
            title={zh ? '刷新' : 'Refresh'}
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
        </div>

        {/* Log entries */}
        <div
          ref={logContainerRef}
          className="rounded-xl overflow-auto font-mono text-xs leading-relaxed"
          style={{
            background: '#0B0E11',
            border: '1px solid #2B3139',
            height: 'calc(100vh - 240px)',
            minHeight: '400px',
          }}
        >
          {entries.length === 0 ? (
            <div className="flex items-center justify-center h-full" style={{ color: '#848E9C' }}>
              {zh ? '暂无日志' : 'No log entries'}
            </div>
          ) : (
            <div className="p-3 space-y-0.5">
              {entries.map(entry => (
                <LogLine key={entry.id} entry={entry} />
              ))}
            </div>
          )}
        </div>

        {/* Status bar */}
        <div className="flex items-center justify-between text-xs" style={{ color: '#848E9C' }}>
          <span>
            {entries.length} {zh ? '条日志' : 'entries'}
            {category && ` | ${zh ? '分类' : 'Category'}: ${category}`}
            {level && ` | ${zh ? '级别' : 'Level'}: ${level.toUpperCase()}`}
          </span>
          <span>
            {autoRefresh
              ? (zh ? '每 2 秒自动刷新' : 'Auto-refresh: 2s')
              : (zh ? '自动刷新已暂停' : 'Auto-refresh paused')}
          </span>
        </div>
      </div>

      {/* Click outside to close export dropdown */}
      {showExport && (
        <div className="fixed inset-0 z-40" onClick={() => setShowExport(false)} />
      )}
    </DeepVoidBackground>
  )
}

function LogLine({ entry }: { entry: LogEntry }) {
  const levelColor = LEVEL_COLORS[entry.level] || '#848E9C'
  const catColor = CATEGORY_COLORS[entry.category] || '#848E9C'

  return (
    <div className="flex gap-2 py-0.5 hover:bg-white/[0.02] rounded px-1 -mx-1 group">
      <span style={{ color: '#585E68' }} className="shrink-0 select-none">
        {entry.timestamp.slice(11)}
      </span>
      <span
        className="shrink-0 w-11 text-right select-none"
        style={{ color: levelColor }}
      >
        [{entry.level.slice(0, 4)}]
      </span>
      <span
        className="shrink-0 opacity-0 group-hover:opacity-100 transition-opacity select-none"
        style={{ color: catColor, fontSize: '10px' }}
      >
        {entry.category}
      </span>
      <span style={{ color: '#EAECEF' }} className="break-all">
        {entry.message}
      </span>
      {entry.source && (
        <span
          className="shrink-0 ml-auto opacity-0 group-hover:opacity-60 transition-opacity text-right select-none"
          style={{ color: '#585E68', fontSize: '10px' }}
        >
          {entry.source}
        </span>
      )}
    </div>
  )
}
