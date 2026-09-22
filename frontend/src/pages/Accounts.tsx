import { useState } from 'react'
import { useAccounts, useDevices, useWebSocket } from '../hooks'
import { api, SIPAccount } from '../api'

export default function Accounts() {
  const { accounts, loading, error, create, update, remove, refetch } = useAccounts()
  const { devices, loading: devicesLoading } = useDevices()
  const [showModal, setShowModal] = useState(false)
  const [editingAccount, setEditingAccount] = useState<SIPAccount | null>(null)
  const [formData, setFormData] = useState<Partial<SIPAccount>>({
    username: '',
    password: '',
    display_name: '',
    device_id: '',
    enabled: true,
  })

  useWebSocket((event) => {
    if (['account_created', 'account_updated', 'account_deleted'].includes(event)) {
      refetch()
    }
  })

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      if (editingAccount) {
        await update(editingAccount.id, formData)
      } else {
        await create(formData)
      }
      closeModal()
    } catch (e) {
      alert(e instanceof Error ? e.message : '操作失败')
    }
  }

  const openCreateModal = () => {
    setEditingAccount(null)
    setFormData({
      username: '',
      password: '',
      display_name: '',
      device_id: '',
      enabled: true,
    })
    setShowModal(true)
  }

  const openEditModal = (account: SIPAccount) => {
    setEditingAccount(account)
    setFormData({
      username: account.username,
      password: '',
      display_name: account.display_name,
      device_id: account.device_id,
      enabled: account.enabled,
    })
    setShowModal(true)
  }

  const closeModal = () => {
    setShowModal(false)
    setEditingAccount(null)
  }

  const confirmDelete = async (account: SIPAccount) => {
    if (window.confirm(`确定要删除账号 "${account.username}" 吗？`)) {
      try {
        await remove(account.id)
      } catch (e) {
        alert(e instanceof Error ? e.message : '删除失败')
      }
    }
  }

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
      <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24}}>
        <h1 style={{fontSize: 24, fontWeight: 600}}>SIP 账号管理</h1>
        <button className="btn btn-primary" onClick={openCreateModal}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
            <line x1={12} y1={5} x2={12} y2={19} />
            <line x1={5} y1={12} x2={19} y2={12} />
          </svg>
          添加账号
        </button>
      </div>

      {error && (
        <div className="card" style={{borderColor: 'var(--danger)', background: 'rgba(239, 68, 68, 0.05)'}}>
          <div style={{color: 'var(--danger)'}}>{error}</div>
        </div>
      )}

      <div className="card">
        {accounts.length === 0 ? (
          <div className="empty-state" style={{padding: 64}}>
            <svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" fill="none" viewBox="0 0 24 24" stroke="currentColor" style={{opacity: 0.3}}>
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
            </svg>
            <h3 style={{marginBottom: 8, color: 'var(--text-primary)'}}>暂无 SIP 账号</h3>
            <p style={{marginBottom: 16}}>点击"添加账号"创建第一个 SIP 账号</p>
            <button className="btn btn-primary" onClick={openCreateModal}>添加账号</button>
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>用户名</th>
                  <th>显示名称</th>
                  <th>关联设备</th>
                  <th>状态</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {accounts.map(account => (
                  <tr key={account.id}>
                    <td>
                      <code style={{fontSize: 13}}>{account.username}@{account.auth_username || 'domain'}</code>
                    </td>
                    <td>{account.display_name || '-'}</td>
                    <td>
                      {devices.find(d => d.id === account.device_id)?.name || account.device_id || '-'}
                    </td>
                    <td>
                      <span className={`badge ${account.enabled ? 'badge-success' : 'badge-info'}`}>
                        {account.enabled ? '已启用' : '已禁用'}
                      </span>
                    </td>
                    <td style={{fontSize: 13, color: 'var(--text-secondary)'}}>
                      {new Date(account.created_at).toLocaleString()}
                    </td>
                    <td>
                      <div style={{display: 'flex', gap: 8}}>
                        <button
                          className="btn btn-sm"
                          onClick={() => openEditModal(account)}
                          title="编辑"
                        >
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
                            <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                            <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                          </svg>
                        </button>
                        <button
                          className="btn btn-sm btn-danger"
                          onClick={() => confirmDelete(account)}
                          title="删除"
                        >
                          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
                            <polyline points="3 6 5 6 21 6" />
                            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                          </svg>
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {showModal && (
        <div className="modal-overlay" onClick={closeModal}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h2 className="modal-title">{editingAccount ? '编辑账号' : '添加账号'}</h2>
              <button className="modal-close" onClick={closeModal}>&times;</button>
            </div>
            <form onSubmit={handleSubmit}>
              <div className="modal-body">
                <div className="form-group">
                  <label className="label">SIP 用户名 *</label>
                  <input
                    type="text"
                    className="input"
                    value={formData.username || ''}
                    onChange={e => setFormData({...formData, username: e.target.value})}
                    placeholder="例如: 1001"
                    required
                    disabled={!!editingAccount}
                  />
                  <p style={{fontSize: 12, color: 'var(--text-muted)', marginTop: 4}}>
                    格式: 用户名@域名 (域名默认为配置的 domain)
                  </p>
                </div>
                
                <div className="form-group">
                  <label className="label">认证用户名</label>
                  <input
                    type="text"
                    className="input"
                    value={formData.auth_username || ''}
                    onChange={e => setFormData({...formData, auth_username: e.target.value})}
                    placeholder="可选，默认同用户名"
                  />
                </div>
                
                <div className="form-group">
                  <label className="label">密码 *</label>
                  <input
                    type="password"
                    className="input"
                    value={formData.password || ''}
                    onChange={e => setFormData({...formData, password: e.target.value})}
                    placeholder={editingAccount ? '留空保持不变' : '输入 SIP 密码'}
                    required={!editingAccount}
                  />
                </div>
                
                <div className="form-group">
                  <label className="label">显示名称</label>
                  <input
                    type="text"
                    className="input"
                    value={formData.display_name || ''}
                    onChange={e => setFormData({...formData, display_name: e.target.value})}
                    placeholder="来电显示名称"
                  />
                </div>
                
                <div className="form-group">
                  <label className="label">关联设备 *</label>
                  <select
                    className="input select"
                    value={formData.device_id || ''}
                    onChange={e => setFormData({...formData, device_id: e.target.value})}
                    required
                    disabled={devicesLoading}
                  >
                    <option value="">选择设备...</option>
                    {devicesLoading ? (
                      <option disabled>加载中...</option>
                    ) : devices.map(device => (
                      <option key={device.id} value={device.id}>
                        {device.name} ({device.id})
                      </option>
                    ))}
                  </select>
                  <p style={{fontSize: 12, color: 'var(--text-muted)', marginTop: 4}}>
                    选择用于拨打/接收电话的蜂窝调制解调器设备
                  </p>
                </div>
                
                <div className="form-group">
                  <label style={{display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer'}}>
                    <input
                      type="checkbox"
                      checked={formData.enabled}
                      onChange={e => setFormData({...formData, enabled: e.target.checked})}
                    />
                    <span>启用此账号</span>
                  </label>
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn" onClick={closeModal}>取消</button>
                <button type="submit" className="btn btn-primary">
                  {editingAccount ? '保存' : '创建'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}