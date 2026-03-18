import { API_BASE, httpClient } from './helpers'

export interface LogEntry {
  id: number
  timestamp: string
  level: string
  source: string
  category: string
  message: string
}

export interface LogResponse {
  entries: LogEntry[]
  latest_id: number
}

export const logApi = {
  async getLogs(
    sinceId?: number,
    limit?: number,
    level?: string,
    category?: string
  ): Promise<LogResponse> {
    const params = new URLSearchParams()
    if (sinceId) params.set('since_id', String(sinceId))
    if (limit) params.set('limit', String(limit))
    if (level) params.set('level', level)
    if (category) params.set('category', category)

    const result = await httpClient.get<LogResponse>(
      `${API_BASE}/logs?${params.toString()}`
    )
    if (!result.success) throw new Error('Failed to fetch logs')
    return result.data!
  },

  async getLogDates(): Promise<string[]> {
    const result = await httpClient.get<string[]>(`${API_BASE}/logs/dates`)
    if (!result.success) return []
    return result.data || []
  },

  getExportUrl(
    format: 'csv' | 'json',
    category?: string,
    date?: string,
    level?: string
  ): string {
    const params = new URLSearchParams({ format })
    if (category) params.set('category', category)
    if (date) params.set('date', date)
    if (level) params.set('level', level)
    const token = localStorage.getItem('auth_token')
    if (token) params.set('token', token)
    return `${API_BASE}/logs/export?${params.toString()}`
  },
}
