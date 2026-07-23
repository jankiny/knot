import { useMemo, useState } from 'react'
import { message } from 'antd'
import { archiveApi } from '../services/api'
import { getDepartments, getProjects, getSettings } from '../services/settings'
import { normalizePathKey, optimizeRecursiveScanDirectories } from '../services/path'

export function buildDefaultReportDirectories() {
  const settings = getSettings()
  const list = []

  if (settings.folderPath) {
    list.push({
      id: 'work-dir',
      label: '当前工作目录',
      path: settings.folderPath,
      checked: true,
      builtin: true
    })
  }

  if (settings.scanPath && normalizePathKey(settings.scanPath) !== normalizePathKey(settings.folderPath)) {
    list.push({
      id: 'scan-dir',
      label: '扫描工作目录',
      path: settings.scanPath,
      checked: true,
      builtin: true
    })
  }

  getDepartments().forEach((dept) => {
    if (!dept.archivePath) return
    list.push({
      id: `dept-${dept.id}`,
      label: `${dept.name}归档目录`,
      path: dept.archivePath,
      checked: true,
      builtin: true
    })
  })

  getProjects().forEach((project) => {
    if (!project.archivePath) return
    list.push({
      id: `project-${project.id}`,
      label: `${project.name}归档目录`,
      path: project.archivePath,
      checked: true,
      builtin: true
    })
  })

  const dedup = new Map()
  list.forEach((item) => {
    const key = normalizePathKey(item.path)
    if (!key || dedup.has(key)) return
    dedup.set(key, item)
  })

  return Array.from(dedup.values())
}

function getFolderSearchText(folder) {
  return `${folder.title || ''} ${folder.name || ''} ${folder.path || ''} ${folder.department || ''} ${folder.project || ''}`.toLowerCase()
}

function getSortTime(folder, preferTaskDate) {
  const value = preferTaskDate ? (folder.task_date || folder.create_time) : folder.create_time
  return new Date(value || 0).getTime()
}

export function useTaskDirectories({ preferTaskDate = false, onScanComplete } = {}) {
  const [directories, setDirectories] = useState(buildDefaultReportDirectories())
  const [scanLoading, setScanLoading] = useState(false)
  const [folders, setFolders] = useState([])
  const [selectedFolderPaths, setSelectedFolderPaths] = useState({})
  const [searchText, setSearchText] = useState('')

  const filteredFolders = useMemo(() => {
    const keyword = searchText.trim().toLowerCase()
    if (!keyword) return folders
    return folders.filter((folder) => getFolderSearchText(folder).includes(keyword))
  }, [folders, searchText])

  const selectedFolders = useMemo(
    () => folders.filter((folder) => !!selectedFolderPaths[folder.path]),
    [folders, selectedFolderPaths]
  )

  const toggleDirectory = (id, checked) => {
    setDirectories((prev) => prev.map((item) => (item.id === id ? { ...item, checked } : item)))
  }

  const removeDirectory = (id) => {
    setDirectories((prev) => prev.filter((item) => item.id !== id))
  }

  const addDirectory = async () => {
    if (!window.electronAPI?.selectFolder) {
      message.info('请在 Electron 客户端中选择目录')
      return
    }

    const path = await window.electronAPI.selectFolder()
    if (!path) return

    const key = normalizePathKey(path)
    const exists = directories.some((item) => normalizePathKey(item.path) === key)
    if (exists) {
      message.info('该目录已存在')
      return
    }

    setDirectories((prev) => [
      ...prev,
      {
        id: `custom-${Date.now()}`,
        label: '自定义目录',
        path,
        checked: true,
        builtin: false
      }
    ])
  }

  const scanFolders = async () => {
    const scanTargets = optimizeRecursiveScanDirectories(directories)
    if (scanTargets.length === 0) {
      message.warning('请至少选择一个扫描目录')
      return
    }

    setScanLoading(true)
    try {
      const results = await Promise.all(
        scanTargets.map(async (item) => {
          try {
            const resp = await archiveApi.scan(item.path, true)
            return { ok: true, item, resp }
          } catch (error) {
            return { ok: false, item, error }
          }
        })
      )

      const folderMap = new Map()
      let failedCount = 0

      results.forEach((result) => {
        if (!result.ok || !result.resp?.success) {
          failedCount += 1
          return
        }

        ;(result.resp.folders || []).forEach((folder) => {
          const pathKey = normalizePathKey(folder?.path)
          if (!pathKey) return

          const existing = folderMap.get(pathKey)
          if (!existing) {
            folderMap.set(pathKey, {
              ...folder,
              fromDirectories: [result.item.label]
            })
            return
          }

          existing.fromDirectories = Array.from(
            new Set([...(existing.fromDirectories || []), result.item.label])
          )
        })
      })

      const folderList = Array.from(folderMap.values()).sort((a, b) => (
        getSortTime(b, preferTaskDate) - getSortTime(a, preferTaskDate)
      ))

      setFolders(folderList)
      setSelectedFolderPaths(Object.fromEntries(folderList.map((folder) => [folder.path, true])))
      onScanComplete?.(folderList)

      if (failedCount > 0) {
        message.warning(`扫描完成，成功 ${folderList.length} 项，失败目录 ${failedCount} 个`)
      } else {
        message.success(`扫描完成，共发现 ${folderList.length} 个任务`)
      }
    } finally {
      setScanLoading(false)
    }
  }

  const setAllFolderChecked = (checked) => {
    const next = {}
    filteredFolders.forEach((folder) => {
      next[folder.path] = checked
    })
    setSelectedFolderPaths((prev) => ({ ...prev, ...next }))
  }

  return {
    addDirectory,
    directories,
    filteredFolders,
    folders,
    removeDirectory,
    scanFolders,
    scanLoading,
    searchText,
    selectedFolderPaths,
    selectedFolders,
    setAllFolderChecked,
    setSearchText,
    setSelectedFolderPaths,
    toggleDirectory
  }
}
