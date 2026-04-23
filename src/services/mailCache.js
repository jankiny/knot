const MAIL_CACHE_KEY = 'knot_mail_cache'
const MAIL_CACHE_VERSION = 1

const DEFAULT_MAIL_LIMIT = 50
const DEFAULT_MAIL_DAYS = 7

function normalizeString(value) {
  return String(value || '').trim()
}

function normalizeIdentityValue(value) {
  return normalizeString(value).toLowerCase()
}

function resolveMailLimit(settings = {}) {
  const value = Number(settings.mailLimit)
  if (Number.isFinite(value) && value > 0) {
    return value
  }
  return DEFAULT_MAIL_LIMIT
}

function resolveMailDays(settings = {}) {
  if (settings.mailDays === undefined || settings.mailDays === null) {
    return DEFAULT_MAIL_DAYS
  }
  const value = Number(settings.mailDays)
  if (Number.isFinite(value) && value >= 0) {
    return value
  }
  return DEFAULT_MAIL_DAYS
}

function buildIdentity(settings = {}) {
  return {
    server: normalizeIdentityValue(settings.mailServer),
    username: normalizeIdentityValue(settings.mailUsername),
    port: Number(settings.mailPort) || 993,
    useSsl: settings.mailUseSsl !== false
  }
}

function isSameIdentity(a, b) {
  return (
    a.server === b.server &&
    a.username === b.username &&
    a.port === b.port &&
    a.useSsl === b.useSsl
  )
}

export function readMailCache(settings = {}) {
  try {
    const raw = localStorage.getItem(MAIL_CACHE_KEY)
    if (!raw) return null

    const parsed = JSON.parse(raw)
    if (!parsed || parsed.version !== MAIL_CACHE_VERSION || !Array.isArray(parsed.mails)) {
      return null
    }

    const expectedIdentity = buildIdentity(settings)
    if (!isSameIdentity(parsed.identity || {}, expectedIdentity)) {
      return null
    }

    const expectedLimit = resolveMailLimit(settings)
    const expectedDays = resolveMailDays(settings)
    if (parsed.mailLimit !== expectedLimit || parsed.mailDays !== expectedDays) {
      return null
    }

    return {
      mails: parsed.mails,
      cachedAt: Number(parsed.cachedAt) || 0,
      mailLimit: parsed.mailLimit,
      mailDays: parsed.mailDays
    }
  } catch (error) {
    console.error('读取邮件缓存失败:', error)
    return null
  }
}

export function saveMailCache(settings = {}, mails = []) {
  try {
    const payload = {
      version: MAIL_CACHE_VERSION,
      identity: buildIdentity(settings),
      mailLimit: resolveMailLimit(settings),
      mailDays: resolveMailDays(settings),
      cachedAt: Date.now(),
      mails: Array.isArray(mails) ? mails : []
    }

    localStorage.setItem(MAIL_CACHE_KEY, JSON.stringify(payload))
    return payload
  } catch (error) {
    console.error('保存邮件缓存失败:', error)
    return null
  }
}
