import { useState } from 'react'
import { useAccounts, useCalls, useWebSocket } from '../hooks'
import { formatDistanceToNow } from '../utils'

export default function Dashboard() {
  const { accounts, loading: accountsLoading } = useAccounts()
  const { calls, loading: callsLoading } = useCalls()
  const [activeCalls, setActiveCalls] = useState<Set<string>>(new Set())

  useWebSocket((event, data) => {
    if (event === 'call_incoming' || event === 'call_answered') {
      setActiveCalls(prev => new Set([...prev, data.call_id]))
    } else if (event === 'call_ended') {
      setActiveCalls(prev => {
        const next = new Set(prev)
        next.delete(data.call_id)
        return next
      })
    }
  })

  const enabledAccounts = accounts.filter(a => a.enabled).length
  const activeCallsCount = calls.filter(c => 
    ['trying', 'ringing', 'in-progress'].includes(c.state)
  ).length
  const todayCalls = calls.filter(c => {
    const callDate = new Date(c.start_time)
    const today = new Date()
    return callDate.toDateString() === today.toDateString()
  }).length

  if (accountsLoading || callsLoading) {
    return (
      <div className="container">
        <div className="stats-grid">
          {[1,2,3,4].map(i => (
            <div key={i} className="stat-card">
              <div className="loading-spinner" style={{margin: '0 auto'}}></div>
            </div>
          ))}
        </div>
      </div>
    )
  }

  const recentCalls = [...calls]
    .sort((a, b) => new Date(b.start_time).getTime() - new Date(a.start_time).getTime())
    .slice(0, 10)

  return (
    <div className="container">
      <h1 style={{marginBottom: 24, fontSize: 24, fontWeight: 600}}>SIP 仪表盘</h1>
      
      <div className="stats-grid">
        <div className="stat-card">
          <div className="stat-value">{accounts.length}</div>
          <div className="stat-label">SIP 账号总数</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">{enabledAccounts}</div>
          <div className="stat-label">已启用账号</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">{activeCallsCount}</div>
          <div className="stat-label">活跃通话</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">{todayCalls}</div>
          <div className="stat-label">今日通话</div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <h2 className="card-title">最近通话记录</h2>
          <a href="/call-log" className="btn btn-sm">查看全部</a>
        </div>
        
        {recentCalls.length === 0 ? (
          <div className="empty-state">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z" />
            </svg>
            <p>暂无通话记录</p>
            <p style={{fontSize: 13, marginTop: 8}}>配置 SIP 账号后开始拨打电话</p>
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
                </tr>
              </thead>
              <tbody>
                {recentCalls.map(call => (
                  <tr key={call.id}>
                    <td>{formatDistanceToNow(new Date(call.start_time))}</td>
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
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {activeCallsCount > 0 && (
        <div className="card" style={{borderColor: 'var(--accent)', borderWidth: 2}}>
          <div className="card-header">
            <h2 className="card-title" style={{color: 'var(--accent)'}}>
              进行中通话 ({activeCallsCount})
            </h2>
          </div>
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>通话 ID</th>
                  <th>方向</th>
                  <th>主叫</th>
                  <th>被叫</th>
                  <th>状态</th>
                  <th>时长</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {calls.filter(c => activeCalls.has(c.id)).map(call => (
                  <tr key={call.id}>
                    <td>{call.id}</td>
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
                    <td>{call.answer_time ? formatDuration(call.duration) : '振铃中...'}</td>
                    <td>
                      {call.state === 'ringing' && (
                        <button className="btn btn-sm btn-primary" onClick={() => {}}>
                          接听
                        </button>
                      )}
                      {call.state === 'in-progress' && (
                        <button className="btn btn-sm btn-danger" onClick={() => {}}>
                          挂断
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
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
    failed: '失败',
    canceled: '已取消',
  }
  return labels[state] || state
}

function formatDuration(seconds: number) {
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

function formatDistanceToNow(date: Date) {
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  const diffHours = Math.floor(diffMs / 3600000)
  const diffDays = Math.floor(diffMs / 86400000)
  
  if (diffMins < 1) return '刚刚'
  if (diffMins < 60) return `${diffMins}分钟前`
  if (diffHours < 24) return `${diffHours}小时前`
  if (diffDays < 7) return `${diffDays}天前`
  return date.toLocaleDateString()
}