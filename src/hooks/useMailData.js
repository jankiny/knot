import { useEffect, useRef, useState } from 'react'
import { mailApi } from '../services/api'
import { getSettings } from '../services/settings'
import { readMailCache, saveMailCache } from '../services/mailCache'

function classifyMailError(error) {
  if (error.code === 'ERR_NETWORK') return 'network'
  if (error.response?.status === 400) return 'not_configured'
  if (error.response?.status === 401 || error.response?.status === 403) return 'auth'
  return 'unknown'
}

async function connectConfiguredMailbox(settings) {
  if (!settings.mailServer || !settings.mailUsername || !settings.mailPasswordEncrypted) return

  try {
    let password = ''
    if (window.electronAPI?.decryptPassword) {
      password = await window.electronAPI.decryptPassword(settings.mailPasswordEncrypted) || ''
    }
    if (!password) return

    await mailApi.connect({
      server: settings.mailServer,
      port: settings.mailPort || 993,
      username: settings.mailUsername,
      password,
      use_ssl: settings.mailUseSsl !== false,
      insecure_skip_verify: settings.mailUseSsl !== false && settings.mailInsecureSkipVerify === true
    })
  } catch (error) {
    console.error('自动连接邮箱失败:', error)
  }
}

export function useMailData() {
  const [mails, setMails] = useState([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [lastRefreshAt, setLastRefreshAt] = useState(0)
  const [fetchDays, setFetchDays] = useState(7)
  const [connectionError, setConnectionError] = useState(null)
  const fetchIdRef = useRef(0)

  const fetchMails = async ({ pageLoading = false, showHint = true, onRefreshed } = {}) => {
    const currentFetchId = ++fetchIdRef.current
    if (pageLoading) {
      setLoading(true)
    } else {
      setRefreshing(true)
    }
    setConnectionError(null)

    try {
      const settings = getSettings()
      await connectConfiguredMailbox(settings)

      if (currentFetchId !== fetchIdRef.current) return

      const limit = settings.mailLimit || 50
      const days = settings.mailDays !== undefined ? settings.mailDays : 7
      setFetchDays(days)

      const result = await mailApi.getMailList(limit, days)
      if (currentFetchId !== fetchIdRef.current) return

      const mailData = result.data || []
      setMails(mailData)
      const saved = saveMailCache(settings, mailData)
      const refreshedAt = saved?.cachedAt || Date.now()
      setLastRefreshAt(refreshedAt)
      if (showHint) {
        onRefreshed?.(refreshedAt)
      }
    } catch (error) {
      if (currentFetchId !== fetchIdRef.current) return
      setConnectionError(classifyMailError(error))
      if (pageLoading) {
        setMails([])
      }
    } finally {
      if (currentFetchId === fetchIdRef.current) {
        setLoading(false)
        setRefreshing(false)
      }
    }
  }

  useEffect(() => {
    const settings = getSettings()
    const cached = readMailCache(settings)

    if (cached) {
      setMails(cached.mails || [])
      setFetchDays(cached.mailDays)
      setLastRefreshAt(cached.cachedAt || 0)
      setLoading(false)
      fetchMails()
      return
    }

    fetchMails({ pageLoading: true, showHint: false })
  }, [])

  return {
    connectionError,
    fetchDays,
    fetchMails,
    lastRefreshAt,
    loading,
    mails,
    refreshing,
    setLastRefreshAt,
    setMails
  }
}
