import { useState } from 'react'
import { useSettings, useWebSocket } from '../hooks'

export default function Settings() {
  const { settings, loading, update } = useSettings()
  const [saved, setSaved] = useState(false)
  const [formData, setFormData] = useState({
    listen_addr: '0.0.0.0',
    sip_port: 5060,
    rtp_port_start: 10000,
    rtp_port_end: 10200,
    domain: 'vocat.local',
    realm: 'VoCat SIP',
    enable_tls: false,
    tls_cert_path: '',
    tls_key_path: '',
  })

  useWebSocket((event) => {
    if (event === 'settings_updated') {
      // Settings updated from another client
    }
  })

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await update(formData)
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    } catch (e) {
      alert(e instanceof Error ? e.message : '保存失败')
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

  // Initialize form data from settings
  if (settings && Object.keys(formData).every(k => formData[k as keyof typeof formData] === (settings[k as keyof typeof settings] ?? formData[k as keyof typeof formData]))) {
    setFormData(prev => ({...prev, ...settings}))
  }

  return (
    <div className="container">
      <h1 style={{marginBottom: 24, fontSize: 24, fontWeight: 600}}>SIP 服务器设置</h1>

      {saved && (
        <div className="card" style={{borderColor: 'var(--success)', background: 'rgba(16, 185, 129, 0.05)', marginBottom: 16}}>
          <div style={{color: 'var(--success)', display: 'flex', alignItems: 'center', gap: 8}}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
              <polyline points="22 4 12 14.01 9 11.01" />
            </svg>
            设置已保存
          </div>
        </div>
      )}

      <div className="card">
        <form onSubmit={handleSubmit}>
          <div className="card-header">
            <h2 className="card-title">网络设置</h2>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label className="label">监听地址</label>
              <input
                type="text"
                className="input"
                value={formData.listen_addr}
                onChange={e => setFormData({...formData, listen_addr: e.target.value})}
                placeholder="0.0.0.0"
              />
            </div>
            <div className="form-group">
              <label className="label">SIP 端口</label>
              <input
                type="number"
                className="input"
                value={formData.sip_port}
                onChange={e => setFormData({...formData, sip_port: parseInt(e.target.value) || 5060})}
                min={1}
                max={65535}
              />
            </div>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label className="label">RTP 起始端口</label>
              <input
                type="number"
                className="input"
                value={formData.rtp_port_start}
                onChange={e => setFormData({...formData, rtp_port_start: parseInt(e.target.value) || 10000})}
                min={1024}
                max={65535}
              />
            </div>
            <div className="form-group">
              <label className="label">RTP 结束端口</label>
              <input
                type="number"
                className="input"
                value={formData.rtp_port_end}
                onChange={e => setFormData({...formData, rtp_port_end: parseInt(e.target.value) || 10200})}
                min={1024}
                max={65535}
              />
            </div>
          </div>

          <div className="card-header" style={{marginTop: 24}}>
            <h2 className="card-title">SIP 域设置</h2>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label className="label">SIP 域名</label>
              <input
                type="text"
                className="input"
                value={formData.domain}
                onChange={e => setFormData({...formData, domain: e.target.value})}
                placeholder="vocat.local"
              />
              <p style={{fontSize: 12, color: 'var(--text-muted)', marginTop: 4}}>
                SIP URI 的域名部分，例如: user@vocat.local
              </p>
            </div>
            <div className="form-group">
              <label className="label">认证 Realm</label>
              <input
                type="text"
                className="input"
                value={formData.realm}
                onChange={e => setFormData({...formData, realm: e.target.value})}
                placeholder="VoCat SIP"
              />
            </div>
          </div>

          <div className="card-header" style={{marginTop: 24}}>
            <h2 className="card-title">TLS 设置</h2>
          </div>

          <div className="form-group">
            <label style={{display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer'}}>
              <input
                type="checkbox"
                checked={formData.enable_tls}
                onChange={e => setFormData({...formData, enable_tls: e.target.checked})}
              />
              <span>启用 TLS (SIPS)</span>
            </label>
          </div>

          {formData.enable_tls && (
            <div className="form-row">
              <div className="form-group">
                <label className="label">证书文件路径</label>
                <input
                  type="text"
                  className="input"
                  value={formData.tls_cert_path}
                  onChange={e => setFormData({...formData, tls_cert_path: e.target.value})}
                  placeholder="/etc/vocat/tls/cert.pem"
                />
              </div>
              <div className="form-group">
                <label className="label">私钥文件路径</label>
                <input
                  type="text"
                  className="input"
                  value={formData.tls_key_path}
                  onChange={e => setFormData({...formData, tls_key_path: e.target.value})}
                  placeholder="/etc/vocat/tls/key.pem"
                />
              </div>
            </div>
          )}

          <div className="modal-footer" style={{marginTop: 24, paddingTop: 16, borderTop: '1px solid var(--border-color)'}}>
            <button type="submit" className="btn btn-primary">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
                <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z" />
                <polyline points="17 21 17 13 7 13 7 21" />
                <polyline points="7 3 7 8 15 8" />
              </svg>
              保存设置
            </button>
          </div>
        </form>
      </div>

      <div className="card" style={{marginTop: 16}}>
        <div className="card-header">
          <h2 className="card-title">客户端配置示例</h2>
        </div>
        <div style={{background: 'var(--bg-tertiary)', borderRadius: 'var(--radius)', padding: 16, fontSize: 13, overflow: 'auto'}}>
          <pre style={{margin: 0, fontFamily: 'monospace', color: 'var(--text-primary)'}}>{`# Zoiper / Groundwire 配置示例

服务器地址: {formData.listen_addr === '0.0.0.0' ? 'YOUR_SERVER_IP' : formData.listen_addr}
SIP 端口: {formData.sip_port}
传输协议: {formData.enable_tls ? 'TLS' : 'UDP'}

账号设置:
- 用户名: 1001 (在账号管理中创建)
- 密码: ********
- 域名: {formData.domain}
- 认证用户名: 1001 (可选，默认同用户名)

高级设置:
- Realm: {formData.realm}
- RTP 端口范围: {formData.rtp_port_start}-{formData.rtp_port_end}
- NAT 穿透: 启用 STUN/ICE
- 保持连接: 启用 (建议 30 秒)

编解码器优先级:
1. opus (推荐)
2. G.722
3. PCMU (G.711 u-law)
4. PCMA (G.711 a-law)`}</pre>
        </div>
      </div>

      <div className="card" style={{marginTop: 16}}>
        <div className="card-header">
          <h2 className="card-title">防火墙规则</h2>
        </div>
        <div style={{fontSize: 13, color: 'var(--text-secondary)'}}>
          <p style={{marginBottom: 12}}>请确保以下端口在防火墙中开放：</p>
          <div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))', gap: 12}}>
            <div style={{background: 'var(--bg-tertiary)', padding: 12, borderRadius: 'var(--radius)'}}>
              <strong>SIP 信令:</strong> UDP/{formData.sip_port}
            </div>
            <div style={{background: 'var(--bg-tertiary)', padding: 12, borderRadius: 'var(--radius)'}}>
              <strong>RTP 媒体:</strong> UDP/{formData.rtp_port_start}-{formData.rtp_port_end}
            </div>
            {formData.enable_tls && (
              <div style={{background: 'var(--bg-tertiary)', padding: 12, borderRadius: 'var(--radius)'}}>
                <strong>SIP over TLS:</strong> TCP/{formData.sip_port}
              </div>
            )}
          </div>
          <p style={{marginTop: 16, fontSize: 12}}>
            <strong>注意:</strong> 如果服务器在 NAT 后面，需要配置端口转发或使用 STUN/TURN 服务器。
          </p>
        </div>
      </div>
    </div>
  )
}