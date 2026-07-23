import { useState } from 'react'
import { Button, Card, DatePicker, Input, message, Space, Tag } from 'antd'
import { CopyOutlined, FileTextOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { reportApi } from '../services/api'
import { getReportAiConfig } from '../hooks/useAiConfig'
import { useTaskDirectories } from '../hooks/useTaskDirectories'
import ReportDirectorySelector from './reports/ReportDirectorySelector'
import ReportTaskSelector from './reports/ReportTaskSelector'
import './DailyReport.css'
import './WeeklyReport.css'

const { RangePicker } = DatePicker

function getDefaultWeekRange() {
  const today = dayjs()
  const monday = today.startOf('day').subtract((today.day() + 6) % 7, 'day')
  return [monday, monday.add(6, 'day')]
}

function WeeklyReport() {
  const [generateLoading, setGenerateLoading] = useState(false)
  const [selectedRange, setSelectedRange] = useState(getDefaultWeekRange())
  const [report, setReport] = useState(null)
  const [markdown, setMarkdown] = useState('')
  const {
    addDirectory,
    directories,
    filteredFolders,
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
  } = useTaskDirectories({
    preferTaskDate: true,
    onScanComplete: () => {
      setReport(null)
      setMarkdown('')
    }
  })

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
      const aiConfig = await getReportAiConfig({
        featureLabel: '周报生成'
      })
      if (!aiConfig) return

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
      <ReportDirectorySelector
        title="周报生成目录选择"
        directories={directories}
        loading={scanLoading}
        onAddDirectory={addDirectory}
        onRemoveDirectory={removeDirectory}
        onScan={scanFolders}
        onToggleDirectory={toggleDirectory}
      />

      <ReportTaskSelector
        folders={filteredFolders}
        getDateLabel={(folder) => folder.task_date || folder.create_time}
        searchText={searchText}
        selectedFolderPaths={selectedFolderPaths}
        setAllFolderChecked={setAllFolderChecked}
        setSearchText={setSearchText}
        setSelectedFolderPaths={setSelectedFolderPaths}
      />

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
          <Button type="primary" icon={<FileTextOutlined />} loading={generateLoading} onClick={handleGenerate}>
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
