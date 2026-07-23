import { useEffect, useState } from 'react'
import { archiveApi } from '../services/api'
import { generateMailHash, getDepartments, getProjects, getSettings } from '../services/settings'

export function useMailGenerationStatus(mails) {
  const [generatedHashMap, setGeneratedHashMap] = useState({})
  const [mailHashCache, setMailHashCache] = useState({})

  useEffect(() => {
    let cancelled = false

    async function loadGeneratedHashes() {
      try {
        const settings = getSettings()
        const hashCacheEntries = await Promise.all(
          mails.map(async (mail) => {
            const hash = await generateMailHash(mail)
            return [mail.id, hash]
          })
        )
        if (cancelled) return
        setMailHashCache(Object.fromEntries(hashCacheEntries))

        const hashMap = {}
        const scanResult = await archiveApi.scan(settings.scanPath || settings.folderPath)
        if (scanResult.success && scanResult.folders) {
          scanResult.folders.forEach((folder) => {
            if (folder.hash) hashMap[folder.hash] = 'working'
          })
        }

        const archiveTargets = [...getDepartments(), ...getProjects()]
        for (const target of archiveTargets) {
          if (!target.archivePath) continue
          try {
            const archiveResult = await archiveApi.scan(target.archivePath, true)
            if (archiveResult.success && archiveResult.folders) {
              archiveResult.folders.forEach((folder) => {
                if (folder.hash) hashMap[folder.hash] = 'archived'
              })
            }
          } catch {
            // Ignore missing or temporarily unavailable archive directories.
          }
        }

        if (!cancelled) {
          setGeneratedHashMap(hashMap)
        }
      } catch (error) {
        if (!cancelled) {
          console.error('加载已生成状态失败:', error)
        }
      }
    }

    if (mails.length === 0) {
      setMailHashCache({})
      setGeneratedHashMap({})
      return undefined
    }

    loadGeneratedHashes()
    return () => {
      cancelled = true
    }
  }, [mails])

  const getGeneratedStatus = (mail) => {
    const mailHash = mailHashCache[mail.id]
    return mailHash && generatedHashMap[mailHash]
  }

  const markGenerated = (mailHash) => {
    setGeneratedHashMap((prev) => ({ ...prev, [mailHash]: 'working' }))
  }

  return {
    getGeneratedStatus,
    mailHashCache,
    markGenerated
  }
}
