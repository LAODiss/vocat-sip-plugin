const API_BASE = '/api'

class APIError extends Error {
  constructor(public status: number, message: string) {
    super(message)
    this.name = 'APIError'
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
    ...options,
  })

  if (!res.ok) {
    const text = await res.text()
    throw new APIError(res.status, text || res.statusText)
  }

  if (res.status === 204) {
    return undefined as T
  }

  return res.json()
}

export const api = {
  // Accounts
  getAccounts: () => request<SIPAccount[]>('/accounts'),
  getAccount: (id: string) => request<SIPAccount>(`/accounts/${id}`),
  createAccount: (data: Partial<SIPAccount>) => request<SIPAccount>('/accounts', {
    method: 'POST',
    body: JSON.stringify(data),
  }),
  updateAccount: (id: string, data: Partial<SIPAccount>) => request<SIPAccount>(`/accounts/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  }),
  deleteAccount: (id: string) => request<void>(`/accounts/${id}`, {
    method: 'DELETE',
  }),

  // Calls
  getCalls: () => request<Call[]>('/calls'),
  getCall: (id: string) => request<Call>(`/calls/${id}`),
  answerCall: (id: string) => request<Call>(`/calls/${id}/answer`, { method: 'POST' }),
  hangupCall: (id: string) => request<Call>(`/calls/${id}/hangup`, { method: 'POST' }),
  holdCall: (id: string) => request<Call>(`/calls/${id}/hold`, { method: 'POST' }),
  unholdCall: (id: string) => request<Call>(`/calls/${id}/unhold`, { method: 'POST' }),

  // Settings
  getSettings: () => request<SIPConfig>('/settings'),
  updateSettings: (data: Partial<SIPConfig>) => request<SIPConfig>('/settings', {
    method: 'PUT',
    body: JSON.stringify(data),
  }),

  // Devices
  getDevices: () => request<Device[]>('/devices'),
}

export interface SIPAccount {
  id: string
  username: string
  password: string
  display_name: string
  device_id: string
  enabled: boolean
  auth_username?: string
  created_at: string
  updated_at: string
}

export interface Call {
  id: string
  account_id: string
  device_id: string
  direction: 'inbound' | 'outbound'
  from: string
  to: string
  state: 'trying' | 'ringing' | 'in-progress' | 'completed' | 'failed' | 'canceled'
  start_time: string
  answer_time?: string
  end_time?: string
  duration: number
  rtp_port?: number
  remote_rtp_addr?: string
  codec?: string
}

export interface SIPConfig {
  listen_addr: string
  sip_port: number
  rtp_port_start: number
  rtp_port_end: number
  domain: string
  realm: string
  enable_tls: boolean
  tls_cert_path: string
  tls_key_path: string
}

export interface Device {
  id: string
  name: string
  device_type: string
  interface: string
}

export { APIError }