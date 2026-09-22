import { useState, useMemo } from 'react'
import { useCalls, useAccounts, useWebSocket } from '../hooks'

export default function CallLog() {
  const { calls, loading, refetch } = useCalls()
  const { accounts } = useAccounts()
  const [filter, setFilter] = useState<'all' | 'inbound' | 'outbound' | 'missed'>('all')
  const [search, setSearch] = useState('')

  useWebSocket((event) => {
    if (['call_incoming', 'call_answered', 'call_ended'].includes(event)) {
      refetch()
    }
  })

  const filteredCalls = useMemo(() => {
    return calls
      .filter(call => {
        if (filter !== 'all' && call.direction !== filter) return false
        if (filter === 'missed' && call.state !== 'failed' && call.state !== 'canceled') return false
        if (search) {
          const s = search.toLowerCase()
          if (!call.from.toLowerCase().includes(s) &&
              !call.to.toLowerCase().includes(s) &&
              !call.id.toLowerCase().includes(s)) return false
        }
        return true
      })
      .sort((a, b) => new Date(b.start_time).getTime() - new Date(a.start_time).getTime())
  }, [calls, filter, search])

  const stats = useMemo(() => ({
    total: calls.length,
    inbound: calls.filter(c => c.direction === 'inbound').length,
    outbound: calls.filter(c => c.direction === 'outbound').length,
    missed: calls.filter(c => c.state === 'failed' || c.state === 'canceled').length,
    completed: calls.filter(c => c.state === 'completed').length,
    totalDuration: calls.reduce((sum, c) => sum + c.duration, 0),
  }), [calls])

  if (loading) {
    return (
      <div className="container">
        <div className="card">
          <div className="loading-spinner" style={{margin: '0 auto', padding: '48px'}}></div>
        </div>
      </div>
    )
  }

  return (
    <div className="container">
      <h1 style={{marginBottom: 24, fontSize: 24, fontWeight: 600}}>通话记录</h1>

      <div className="card" style={{marginBottom: 16}}>
        <div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: 16}}>
          <div>
            <div className="stat-value">{stats.total}</div>
            <div className="stat-label">总通话数</div>
          </div>
          <div>
            <div className="stat-value">{stats.inbound}</div>
            <div className="stat-label">来电</div>
          </div>
          <div>
            <div className="stat-value">{stats.outbound}</div>
            <div className="stat-label">去电</div>
          </div>
          <div>
            <div className="stat-value">{stats.missed}</div>
            <div className="stat-label">未接来电</div>
          </div>
          <div>
            <div className="stat-value">{stats.completed}</div>
            <div className="stat-label">已接通</div>
          </div>
          <div>
            <div className="stat-value">{formatTotalDuration(stats.totalDuration)}</div>
            <div className="stat-label">总通话时长</div>
          </div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <div style={{display: 'flex', gap: 8, flexWrap: 'wrap'}}>
            {(['all', 'inbound', 'outbound', 'missed'] as const).map(f => (
              <button
                key={f}
                className={`btn ${filter === f ? 'btn-primary' : ''}`}
                onClick={() => setFilter(f)}
                style={{textTransform: 'capitalize'}}
              >
                {f === 'all' ? '全部' : f === 'inbound' ? '来电' : f === 'outbound' ? '去电' : '未接'}
              </button>
            ))}
          </div>
          <input
            type="search"
            className="input"
            placeholder="搜索号码、通话 ID..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            style={{maxWidth: 300}}
          />
        </div>

        {filteredCalls.length === 0 ? (
          <div className="empty-state">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z" />
            </svg>
            <p>{calls.length === 0 ? '暂无通话记录' : '无匹配的通话记录'}</p>
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>时间</th>
                  <th>方向</th>
                  <th>主叫</th>
                  <th>被叫</th>
                  <th>状态</th>
                  <th>时长</th>
                  <th>账号</th>
                  <th>设备</th>
                </tr>
              </thead>
              <tbody>
                {filteredCalls.map(call => (
                  <tr key={call.id}>
                    <td style={{whiteSpace: 'nowrap'}}>
                      {new Date(call.start_time).toLocaleString()}
                    </td>
                    <td>
                      <span className={`badge ${call.direction === 'inbound' ? 'badge-info' : 'badge-success'}`}>
                        {call.direction === 'inbound' ? '来电' : '去电'}
                      </span>
                    </td>
                    <td>{call.from}</td>
                    <td>{call.to}</td>
                    <td>
                      <span className={`badge ${getStateBadge(call.state)}`}>
                        {getStateLabel(call.state)}
                      </span>
                    </td>
                    <td>{call.duration > 0 ? formatDuration(call.duration) : '-'}</td>
                    <td>
                      {accounts.find(a => a.id === call.account_id)?.username || call.account_id}
                    </td>
                    <td style={{fontSize: 13, color: 'var(--text-secondary)'}}>
                      {call.device_id}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}

function getStateBadge(state: string) {
  switch (state) {
    case 'trying': return 'badge-info'
    case 'ringing': return 'badge-warning'
    case 'in-progress': return 'badge-success'
    case 'completed': return 'badge-info'
    case 'failed': return 'badge-danger'
    case 'canceled': return 'badge-info'
    default: return 'badge-info'
  }
}

function getStateLabel(state: string) {
  const labels: Record<string, string> = {
    trying: '尝试中',
    ringing: '振铃中',
    'in-progress': '通话中',
    completed: '已完成',
    failed: '失败/未接',
    canceled: '已取消',
  }
  return labels[state] || state
}

function formatDuration(seconds: number) {
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  if (mins > 59) {
    const hours = Math.floor(mins / 60)
    return `${hours}h ${mins % 60}m`
  }
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

function formatTotalDuration(seconds: number) {
  const hours = Math.floor(seconds / 3600)
  const mins = Math.floor((seconds % 3600) / 60)
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}