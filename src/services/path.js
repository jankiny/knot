export function normalizePathKey(inputPath) {
  if (!inputPath) return ''

  let normalized = String(inputPath).trim()
  if (!normalized) return ''

  normalized = normalized.replace(/\\/g, '/')
  normalized = normalized.replace(/\/+/g, '/')

  if (normalized.length > 1) {
    normalized = normalized.replace(/\/+$/, '')
  }

  // Windows drive-letter and UNC-like paths are case-insensitive.
  if (/^[a-zA-Z]:\//.test(normalized) || normalized.startsWith('//')) {
    normalized = normalized.toLowerCase()
  }

  return normalized
}

export function isSubPath(parentPath, childPath) {
  const parent = normalizePathKey(parentPath)
  const child = normalizePathKey(childPath)
  if (!parent || !child || parent === child) return false
  return child.startsWith(`${parent}/`)
}

export function optimizeRecursiveScanDirectories(directories) {
  const selected = (directories || []).filter((item) => item?.checked && item?.path)
  if (selected.length === 0) return []

  const sorted = [...selected].sort((a, b) => {
    const ak = normalizePathKey(a.path)
    const bk = normalizePathKey(b.path)
    return ak.length - bk.length
  })

  const kept = []
  const seen = new Set()

  for (const item of sorted) {
    const key = normalizePathKey(item.path)
    if (!key || seen.has(key)) continue

    const coveredByParent = kept.some((parent) => {
      const parentKey = normalizePathKey(parent.path)
      return key === parentKey || isSubPath(parentKey, key)
    })
    if (coveredByParent) continue

    kept.push(item)
    seen.add(key)
  }

  return kept
}
