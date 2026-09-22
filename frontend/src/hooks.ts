import { useEffect, useRef, useState, useCallback } from 'react'
import { api, SIPAccount, Call } from './api'

type WSMessage = {
  type: 'state' | 'event' | 'heartbeat'
  event?: string
  data?: any
  accounts?: SIPAccount[]
  calls?: Call[]
  config?: any
  time?: string
}

export function useWebSocket(onEvent?: (event: string, data: any) => void) {
  const [connected, setConnected] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<NodeJS.Timeout>()

  const connect = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws`)
    
    ws.onopen = () => {
      setConnected(true)
      console.log('WebSocket connected')
    }

    ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data)
        if (msg.type === 'event' && msg.event && onEvent) {
          onEvent(msg.event, msg.data)
        }
      } catch (e) {
        console.error('Failed to parse WS message:', e)
      }
    }

    ws.onclose = () => {
      setConnected(false)
      console.log('WebSocket disconnected, reconnecting...')
      reconnectTimeoutRef.current = setTimeout(connect, 3000)
    }

    ws.onerror = (err) => {
      console.error('WebSocket error:', err)
    }

    wsRef.current = ws
  }, [onEvent])

  useEffect(() => {
    connect()
    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
      wsRef.current?.close()
    }
  }, [connect])

  const send = useCallback((data: any) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data))
    }
  }, [])

  return { connected, send }
}

export function useAccounts() {
  const [accounts, setAccounts] = useState<SIPAccount[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchAccounts = useCallback(async () => {
    try {
      setError(null)
      const data = await api.getAccounts()
      setAccounts(data)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load accounts')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchAccounts()
  }, [fetchAccounts])

  const create = async (data: Partial<SIPAccount>) => {
    const acc = await api.createAccount(data)
    setAccounts(prev => [...prev, acc])
    return acc
  }

  const update = async (id: string, data: Partial<SIPAccount>) => {
    const acc = await api.updateAccount(id, data)
    setAccounts(prev => prev.map(a => a.id === id ? acc : a))
    return acc
  }

  const remove = async (id: string) => {
    await api.deleteAccount(id)
    setAccounts(prev => prev.filter(a => a.id !== id))
  }

  return { accounts, loading, error, refetch: fetchAccounts, create, update, remove }
}

export function useCalls() {
  const [calls, setCalls] = useState<Call[]>([])
  const [loading, setLoading] = useState(true)

  const fetchCalls = useCallback(async () => {
    try {
      const data = await api.getCalls()
      setCalls(data)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchCalls()
  }, [fetchCalls])

  return { calls, loading, refetch: fetchCalls }
}

export function useSettings() {
  const [settings, setSettings] = useState<any>(null)
  const [loading, setLoading] = useState(true)

  const fetchSettings = useCallback(async () => {
    try {
      const data = await api.getSettings()
      setSettings(data)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchSettings()
  }, [fetchSettings])

  const update = async (data: any) => {
    const updated = await api.updateSettings(data)
    setSettings(updated)
    return updated
  }

  return { settings, loading, update }
}

export function useDevices() {
  const [devices, setDevices] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  const fetchDevices = useCallback(async () => {
    try {
      const data = await api.getDevices()
      setDevices(data)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchDevices()
  }, [fetchDevices])

  return { devices, loading, refetch: fetchDevices }
}