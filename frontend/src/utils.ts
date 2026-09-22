export function formatDuration(seconds: number): string {
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  if (mins > 59) {
    const hours = Math.floor(mins / 60)
    return `${hours}h ${mins % 60}m`
  }
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

export function formatDistanceToNow(date: Date | string): string {
  const d = new Date(date)
  const now = new Date()
  const diffMs = now.getTime() - d.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  const diffHours = Math.floor(diffMs / 3600000)
  const diffDays = Math.floor(diffMs / 86400000)
  
  if (diffMins < 1) return '刚刚'
  if (diffMins < 60) return `${diffMins}分钟前`
  if (diffHours < 24) return `${diffHours}小时前`
  if (diffDays < 7) return `${diffDays}天前`
  return d.toLocaleDateString()
}

export function getStateLabel(state: string): string {
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

export function getStateBadge(state: string): string {
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

export function generateCallID(): string {
  return `call-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`
}