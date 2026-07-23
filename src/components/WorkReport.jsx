import { useMemo, useState } from 'react'
import { Button, Card, Checkbox, DatePicker, Empty, Input, List, message, Select, Segmented, Space, Tag, Typography } from 'antd'
import { CopyOutlined, FileTextOutlined, FolderOpenOutlined, ReloadOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { reportApi } from '../services/api'
import { getReportAiConfig } from '../hooks/useAiConfig'
import { buildDefaultReportDirectories } from '../hooks/useTaskDirectories'
import { normalizePathKey, optimizeRecursiveScanDirectories } from '../services/path'
import './WorkReport.css'

const { Text } = Typography

const REPORT_TYPES = [
  { label: '日报', value: 'daily' },
  { label: '周报', value: 'weekly' },
  { label: '月报', value: 'monthly' },
  { label: '自定义', value: 'custom' }
]

const REPORT_TYPE_LABEL = {
  daily: '日报',
  weekly: '周报',
  monthly: '月报',
  custom: '工作报告'
}

function getDefaultWeekStart(date) {
  return date.startOf('day').subtract((date.day() + 6) % 7, 'day')
}

function buildRange(reportType, startDate) {
  const start = startDate.startOf('day')
  switch (reportType) {
    case 'daily':
      return [start, start]
    case 'monthly':
      return [start.startOf('month'), start.endOf('month')]
    case 'custom':
      return [start, start]
    default: {
      const weekStart = getDefaultWeekStart(start)
      return [weekStart, weekStart.add(6, 'day')]
    }
  }
}

function WorkReport() {
  const [directories, setDirectories] = useState(() => buildDefaultReportDirectories())
  const [selectedDirectoryIds, setSelectedDirectoryIds] = useState(() => (
    buildDefaultReportDirectories().filter((item) => item.checked).map((item) => item.id)
  ))
  const [reportType, setReportType] = useState('weekly')
  const [startDate, setStartDate] = useState(() => getDefaultWeekStart(dayjs()))
  const [endDate, setEndDate] = useState(() => getDefaultWeekStart(dayjs()).add(6, 'day'))
  const [scanLoading, setScanLoading] = useState(false)
  const [generateLoading, setGenerateLoading] = useState(false)
  const [scanItems, setScanItems] = useState([])
  const [scannedCount, setScannedCount] = useState(0)
  const [selectedTaskPaths, setSelectedTaskPaths] = useState({})
  const [report, setReport] = useState(null)
  const [markdown, setMarkdown] = useState('')

  const directoryOptions = directories.map((item) => ({
    label: item.label,
    value: item.id,
    path: item.path
  }))

  const selectedDirectories = useMemo(() => {
    const selected = new Set(selectedDirectoryIds)
    return directories
      .map((item) => ({ ...item, checked: selected.has(item.id) }))
      .filter((item) => item.checked && item.path)
  }, [directories, selectedDirectoryIds])

  const selectedItems = useMemo(
    () => scanItems.filter((item) => !!selectedTaskPaths[item.folder_path]),
    [scanItems, selectedTaskPaths]
  )

  const applyReportType = (nextType, baseDate = startDate) => {
    const [start, end] = buildRange(nextType, baseDate)
    setReportType(nextType)
    setStartDate(start)
    setEndDate(end)
    setScanItems([])
    setSelectedTaskPaths({})
    setReport(null)
    setMarkdown('')
  }

  const handleStartDateChange = (date) => {
    const nextStart = date || dayjs()
    const [start, end] = buildRange(reportType, nextStart)
    setStartDate(start)
    setEndDate(reportType === 'custom' ? endDate : end)
  }

  const handleAddDirectory = async () => {
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

    const id = `custom-${Date.now()}`
    setDirectories((prev) => [
      ...prev,
      { id, label: '自定义目录', path, checked: true, builtin: false }
    ])
    setSelectedDirectoryIds((prev) => [...prev, id])
  }

  const handleScan = async () => {
    if (!startDate || !endDate || endDate.isBefore(startDate, 'day')) {
      message.warning('请选择有效的报告周期')
      return
    }
    const scanTargets = optimizeRecursiveScanDirectories(selectedDirectories)
    if (scanTargets.length === 0) {
      message.warning('请至少选择一个扫描目录')
      return
    }

    setScanLoading(true)
    setReport(null)
    setMarkdown('')
    try {
      const resp = await reportApi.scanWork({
        period_start: startDate.format('YYYY-MM-DD'),
        period_end: endDate.format('YYYY-MM-DD'),
        scan_paths: scanTargets.map((item) => item.path)
      })
      const items = resp.items || []
      setScanItems(items)
      setScannedCount(resp.scanned_count || 0)
      setSelectedTaskPaths(Object.fromEntries(items.map((item) => [item.folder_path, true])))
      if (items.length === 0) {
        message.info('当前周期未扫描到可纳入报告的任务')
      } else {
        message.success(`扫描完成，命中 ${items.length} 个任务`)
      }
    } catch (error) {
      message.error(error.response?.data?.detail || '扫描任务失败')
    } finally {
      setScanLoading(false)
    }
  }

  const handleGenerate = async () => {
    if (selectedItems.length === 0) {
      message.warning('请至少保留一个任务用于生成报告')
      return
    }

    setGenerateLoading(true)
    try {
      const aiConfig = await getReportAiConfig({
        featureLabel: '工作报告生成'
      })
      if (!aiConfig) return

      const resp = await reportApi.generateWork({
        report_type: reportType,
        period_start: startDate.format('YYYY-MM-DD'),
        period_end: endDate.format('YYYY-MM-DD'),
        items: selectedItems.map((item) => ({ folder_path: item.folder_path })),
        ai: aiConfig
      })
      const outputReport = resp.report || {}
      setReport(outputReport)
      setMarkdown(outputReport.markdown || '')
      message.success(`已基于 ${resp.count || selectedItems.length} 项任务生成${REPORT_TYPE_LABEL[reportType]}`)
    } catch (error) {
      message.error(error.response?.data?.detail || '生成报告失败')
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

  const handleOpenFolder = async (folderPath) => {
    if (!window.electronAPI?.openFolder) {
      message.info('请在 Electron 客户端中打开目录')
      return
    }
    const ok = await window.electronAPI.openFolder(folderPath)
    if (!ok) message.error('打开目录失败')
  }

  const setAllTasksChecked = (checked) => {
    setSelectedTaskPaths(Object.fromEntries(scanItems.map((item) => [item.folder_path, checked])))
  }

  return (
    <div className="work-report">
      <Card title="报告范围" className="work-report-card">
        <div className="work-report-form">
          <div className="work-report-field work-report-directory-field">
            <div className="work-report-label">扫描目录</div>
            <Select
              mode="multiple"
              value={selectedDirectoryIds}
              options={directoryOptions}
              onChange={setSelectedDirectoryIds}
              maxTagCount="responsive"
              placeholder="选择工作目录和归档目录"
              optionRender={(option) => (
                <div className="work-report-directory-option">
                  <span>{option.data.label}</span>
                  <Text type="secondary">{option.data.path}</Text>
                </div>
              )}
            />
          </div>

          <div className="work-report-field work-report-type-field">
            <div className="work-report-label">报告类型</div>
            <Segmented
              value={reportType}
              options={REPORT_TYPES}
              onChange={(value) => applyReportType(value)}
            />
          </div>

          <div className="work-report-field">
            <div className="work-report-label">起始日期</div>
            <DatePicker value={startDate} onChange={handleStartDateChange} allowClear={false} />
          </div>

          <div className="work-report-field">
            <div className="work-report-label">截止日期</div>
            <DatePicker
              value={endDate}
              onChange={(date) => {
                setEndDate(date || startDate)
                setReportType('custom')
              }}
              allowClear={false}
            />
          </div>
        </div>

        <div className="work-report-actions">
          <Button icon={<FolderOpenOutlined />} onClick={handleAddDirectory}>添加目录</Button>
          <Button icon={<ReloadOutlined />} loading={scanLoading} onClick={handleScan}>扫描任务</Button>
          <Button type="primary" icon={<FileTextOutlined />} loading={generateLoading} disabled={selectedItems.length === 0} onClick={handleGenerate}>
            生成报告
          </Button>
        </div>
      </Card>

      <Card
        title="扫描结果"
        className="work-report-card"
        extra={scannedCount > 0 ? <Tag>已检查 {scannedCount} 项</Tag> : null}
      >
        <div className="work-report-result-header">
          <Space wrap>
            <Tag color="green">纳入 {selectedItems.length} 项</Tag>
            <Tag>命中 {scanItems.length} 项</Tag>
          </Space>
          <Space>
            <Button size="small" onClick={() => setAllTasksChecked(true)}>全选</Button>
            <Button size="small" onClick={() => setAllTasksChecked(false)}>清空</Button>
          </Space>
        </div>

        {scanItems.length === 0 ? (
          <Empty description={scanLoading ? '正在扫描任务...' : '选择周期后点击扫描任务'} image={Empty.PRESENTED_IMAGE_SIMPLE} />
        ) : (
          <List
            className="work-report-task-list"
            dataSource={scanItems}
            renderItem={(item) => (
              <List.Item
                actions={[
                  <Button key="open" size="small" icon={<FolderOpenOutlined />} onClick={() => handleOpenFolder(item.folder_path)}>
                    打开
                  </Button>
                ]}
              >
                <div className="work-report-task">
                  <Checkbox
                    checked={!!selectedTaskPaths[item.folder_path]}
                    onChange={(event) => setSelectedTaskPaths((prev) => ({ ...prev, [item.folder_path]: event.target.checked }))}
                  >
                    <span className="work-report-task-title">{item.title || item.folder_name || '未命名任务'}</span>
                  </Checkbox>
                  <div className="work-report-task-meta">
                    {item.task_date && <Tag>{item.task_date}</Tag>}
                    {item.latest_activity && <Tag color="green">活动 {item.latest_activity}</Tag>}
                    {item.department && <Tag color="blue">{item.department}</Tag>}
                    {item.project && <Tag color="purple">{item.project}</Tag>}
                    {(item.match_reasons || []).map((reason) => (
                      <Tag key={`${item.folder_path}-${reason}`} color="default">{reason}</Tag>
                    ))}
                  </div>
                  <div className="work-report-task-path">{item.folder_path}</div>
                </div>
              </List.Item>
            )}
          />
        )}
      </Card>

      <Card title="报告输出" className="work-report-card">
        {report?.overview && (
          <div className="work-report-summary">
            <div className="work-report-summary-title">{report.title || `${REPORT_TYPE_LABEL[reportType]}预览`}</div>
            <div className="work-report-summary-text">{report.overview}</div>
          </div>
        )}
        <div className="work-report-markdown-header">
          <span>Markdown 输出</span>
          <Button icon={<CopyOutlined />} onClick={handleCopy}>复制 Markdown</Button>
        </div>
        <Input.TextArea
          value={markdown}
          readOnly
          autoSize={{ minRows: 10, maxRows: 22 }}
          placeholder="生成后将在这里显示报告"
        />
      </Card>
    </div>
  )
}

export default WorkReport
