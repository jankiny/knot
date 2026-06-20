import { useMemo, useState } from 'react'
import { Button, Card, Checkbox, DatePicker, Empty, Input, List, message, Space, Tag, Typography } from 'antd'
import { CopyOutlined, FolderOpenOutlined, ReloadOutlined, RobotOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { archiveApi, reportApi } from '../services/api'
import { getDepartments, getProjects, getSelectedAiModel, getSettings } from '../services/settings'
import { normalizePathKey, optimizeRecursiveScanDirectories } from '../services/path'
import './DailyReport.css'
import './WeeklyReport.css'

const { Text } = Typography
const { RangePicker } = DatePicker

function getDefaultWeekRange() {
  const today = dayjs()
  const monday = today.startOf('day').subtract((today.day() + 6) % 7, 'day')
  return [monday, monday.add(6, 'day')]
}

function buildDefaultDirectories() {
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

  getDepartments().forEach((dept) => {
    if (!dept.archivePath) return
    list.push({
      id: `dept-${dept.id}`,
      label: `${dept.name}归档目录`,
      path: dept.archivePath,
      checked: false,
      builtin: true
    })
  })

  getProjects().forEach((project) => {
    if (!project.archivePath) return
    list.push({
      id: `project-${project.id}`,
      label: `${project.name}归档目录`,
      path: project.archivePath,
      checked: false,
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

function WeeklyReport() {
  const [directories, setDirectories] = useState(buildDefaultDirectories())
  const [scanLoading, setScanLoading] = useState(false)
  const [generateLoading, setGenerateLoading] = useState(false)
  const [folders, setFolders] = useState([])
  const [selectedFolderPaths, setSelectedFolderPaths] = useState({})
  const [selectedRange, setSelectedRange] = useState(getDefaultWeekRange())
  const [searchText, setSearchText] = useState('')
  const [report, setReport] = useState(null)
  const [markdown, setMarkdown] = useState('')

  const filteredFolders = useMemo(() => {
    const keyword = searchText.trim().toLowerCase()
    if (!keyword) return folders
    return folders.filter((folder) => {
      const text = `${folder.title || ''} ${folder.name || ''} ${folder.path || ''} ${folder.department || ''} ${folder.project || ''}`.toLowerCase()
      return text.includes(keyword)
    })
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

  const handleScan = async () => {
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

      const folderList = Array.from(folderMap.values()).sort((a, b) => {
        const aTime = new Date(a.task_date || a.create_time || 0).getTime()
        const bTime = new Date(b.task_date || b.create_time || 0).getTime()
        return bTime - aTime
      })

      setFolders(folderList)
      setSelectedFolderPaths(Object.fromEntries(folderList.map((f) => [f.path, true])))
      setReport(null)
      setMarkdown('')

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

  const handleGenerate = async () => {
    if (!selectedRange?.[0] || !selectedRange?.[1]) {
      message.warning('请选择周报周期')
      return
    }
    if (selectedFolders.length === 0) {
      message.warning('请至少勾选一个任务')
      return
    }

    setGenerateLoading(true)
    try {
      const settings = getSettings()
      const selectedModel = getSelectedAiModel()
      const aiConfig = {
        enabled: !!settings.enableAiWeeklyReport,
        api_url: selectedModel?.apiUrl || '',
        api_key: '',
        model: selectedModel?.modelId || ''
      }

      if (aiConfig.enabled) {
        if (!selectedModel || !aiConfig.api_url || !aiConfig.model || !selectedModel.apiKeyEncrypted) {
          message.warning('AI 周报已启用，但当前模型的 API 地址、模型 ID 或 API Key 未完整配置')
          return
        }

        if (window.electronAPI?.decryptPassword) {
          aiConfig.api_key = await window.electronAPI.decryptPassword(selectedModel.apiKeyEncrypted) || ''
        }
        if (!aiConfig.api_key) {
          message.warning('AI Key 解密失败，请重新保存 AI Key')
          return
        }
      }

      const req = {
        period_start: selectedRange[0].format('YYYY-MM-DD'),
        period_end: selectedRange[1].format('YYYY-MM-DD'),
        items: selectedFolders.map((folder) => ({
          folder_path: folder.path,
          work_record: ''
        })),
        ai: aiConfig
      }

      const resp = await reportApi.generateWeekly(req)
      if (!resp.success) {
        message.error('周报生成失败')
        return
      }

      const outputReport = resp.report || {}
      setReport(outputReport)
      setMarkdown(outputReport.markdown || '')
      message.success(`已基于 ${resp.count || 0} 项周期内任务生成周报`)
    } catch (error) {
      message.error(error.response?.data?.detail || '周报生成失败')
    } finally {
      setGenerateLoading(false)
    }
  }

  const handleCopy = async () => {
    if (!markdown.trim()) {
      message.info('暂无可复制内容')
      return
    }

    try {
      await navigator.clipboard.writeText(markdown)
      message.success('已复制 Markdown')
    } catch {
      message.error('复制失败')
    }
  }

  return (
    <div className="daily-report weekly-report">
      <Card title="周报生成目录选择" className="daily-card">
        <div className="directory-actions">
          <Button onClick={addDirectory} icon={<FolderOpenOutlined />}>添加目录</Button>
          <Button type="primary" onClick={handleScan} icon={<ReloadOutlined />} loading={scanLoading}>扫描任务</Button>
        </div>

        <div className="directory-list">
          {directories.length === 0 && <Empty description="暂无可用目录" />}
          {directories.map((item) => (
            <div className="directory-item" key={item.id}>
              <Checkbox checked={item.checked} onChange={(e) => toggleDirectory(item.id, e.target.checked)}>
                <span className="directory-label">{item.label}</span>
              </Checkbox>
              <Text type="secondary" className="directory-path">{item.path}</Text>
              {!item.builtin && (
                <Button type="link" danger onClick={() => removeDirectory(item.id)}>移除</Button>
              )}
            </div>
          ))}
        </div>
      </Card>

      <Card title="任务筛选" className="daily-card">
        <div className="task-toolbar">
          <Input
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            placeholder="搜索任务名称 / 路径 / 归属"
            allowClear
          />
          <Space>
            <Button onClick={() => setAllFolderChecked(true)}>全选当前列表</Button>
            <Button onClick={() => setAllFolderChecked(false)}>取消当前列表</Button>
          </Space>
        </div>

        {filteredFolders.length === 0 ? (
          <Empty description="请先扫描任务目录" />
        ) : (
          <List
            dataSource={filteredFolders}
            renderItem={(folder) => (
              <List.Item>
                <div className="task-item">
                  <Checkbox
                    checked={!!selectedFolderPaths[folder.path]}
                    onChange={(e) => setSelectedFolderPaths((prev) => ({ ...prev, [folder.path]: e.target.checked }))}
                  >
                    <span className="task-title">{folder.title || folder.name}</span>
                  </Checkbox>
                  <div className="task-meta">
                    {folder.department && <Tag color="blue">{folder.department}</Tag>}
                    {folder.project && <Tag color="purple">{folder.project}</Tag>}
                    {(folder.task_date || folder.create_time) && <Tag>{folder.task_date || folder.create_time}</Tag>}
                    {(folder.fromDirectories || []).map((dirName) => (
                      <Tag key={`${folder.path}-${dirName}`} color="blue">{dirName}</Tag>
                    ))}
                  </div>
                </div>
              </List.Item>
            )}
          />
        )}
      </Card>

      <Card title="周报生成" className="daily-card">
        <div className="generate-bar">
          <Space wrap>
            <span>周报周期</span>
            <RangePicker
              value={selectedRange}
              onChange={(range) => setSelectedRange(range || getDefaultWeekRange())}
              allowClear={false}
            />
            <Tag color="green">已选任务：{selectedFolders.length}</Tag>
          </Space>
          <Button type="primary" icon={<RobotOutlined />} loading={generateLoading} onClick={handleGenerate}>
            生成周报
          </Button>
        </div>

        {report?.overview && (
          <div className="weekly-summary">
            <div className="weekly-summary-title">{report.title || '周报预览'}</div>
            <div className="weekly-summary-text">{report.overview}</div>
          </div>
        )}

        <div className="markdown-block">
          <div className="markdown-header">
            <span>Markdown 输出</span>
            <Button icon={<CopyOutlined />} onClick={handleCopy}>复制 Markdown</Button>
          </div>
          <Input.TextArea
            value={markdown}
            readOnly
            autoSize={{ minRows: 10, maxRows: 22 }}
            placeholder="生成后将在这里显示完整周报"
          />
        </div>
      </Card>
    </div>
  )
}

export default WeeklyReport
