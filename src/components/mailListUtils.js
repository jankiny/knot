export function formatRefreshTime(timestamp) {
  if (!timestamp) return ''
  try {
    return new Date(timestamp).toLocaleTimeString('zh-CN', {
      hour12: false,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    })
  } catch {
    return ''
  }
}

export function formatMailDate(dateStr) {
  if (!dateStr) return ''
  try {
    const date = new Date(dateStr)
    return date.toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit'
    })
  } catch {
    return dateStr
  }
}

export function formatFileSize(bytes) {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

export function getBodyPreview(body, maxLength = 100) {
  if (!body) return ''
  const text = body.replace(/\n+/g, ' ').trim()
  if (text.length <= maxLength) return text
  return text.slice(0, maxLength) + '...'
}

export function getMailMonthKey(dateStr) {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  if (Number.isNaN(date.getTime())) return ''
  const year = date.getFullYear()
  const month = date.getMonth() + 1
  return `${year}-${month.toString().padStart(2, '0')}`
}

export function buildMailTimeline(mails) {
  if (!mails || mails.length === 0) return []

  const monthMap = new Map()
  mails.forEach((mail) => {
    if (!mail.date) return
    const date = new Date(mail.date)
    if (Number.isNaN(date.getTime())) return

    const year = date.getFullYear()
    const month = date.getMonth() + 1
    const monthKey = `${year}-${month.toString().padStart(2, '0')}`

    if (!monthMap.has(monthKey)) {
      monthMap.set(monthKey, {
        key: monthKey,
        label: `${year}年${month}月`,
        mailId: mail.id,
        timestamp: date.getTime()
      })
    }
  })

  return Array.from(monthMap.values()).sort((a, b) => b.timestamp - a.timestamp)
}
