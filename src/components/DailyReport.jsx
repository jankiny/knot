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

function DailyReport() {
  const [generateLoading, setGenerateLoading] = useState(false)
  const [selectedDate, setSelectedDate] = useState(dayjs())
  const [logs, setLogs] = useState([])
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
    onScanComplete: () => {
      setLogs([])
      setMarkdown('')
    }
  })

  const handleGenerate = async () => {
    if (selectedFolders.length === 0) {
      message.warning('请至少勾选一个任务')
      return
    }

    setGenerateLoading(true)
    try {
      const aiConfig = await getReportAiConfig({
        featureLabel: '日报生成'
      })
      if (!aiConfig) return

      const req = {
        date: selectedDate.format('YYYY-MM-DD'),
        items: selectedFolders.map((folder) => ({
          folder_path: folder.path,
          work_record: ''
        })),
        ai: aiConfig
      }

      const resp = await reportApi.generateDaily(req)
      if (!resp.success) {
        message.error('日报生成失败')
        return
      }

      const outputLogs = resp.logs || []
      const md = outputLogs.map((item) => `- ${item.content}`).join('\n')
      setLogs(outputLogs)
      setMarkdown(md)
      message.success(`已生成 ${outputLogs.length} 条日报日志`)
    } catch (error) {
      message.error(error.response?.data?.detail || '日报生成失败')
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
    <div className="daily-report">
      <ReportDirectorySelector
        title="日报生成目录选择"
        directories={directories}
        loading={scanLoading}
        onAddDirectory={addDirectory}
        onRemoveDirectory={removeDirectory}
        onScan={scanFolders}
        onToggleDirectory={toggleDirectory}
      />

      <ReportTaskSelector
        folders={filteredFolders}
        getDateLabel={(folder) => folder.create_time}
        searchText={searchText}
        selectedFolderPaths={selectedFolderPaths}
        setAllFolderChecked={setAllFolderChecked}
        setSearchText={setSearchText}
        setSelectedFolderPaths={setSelectedFolderPaths}
      />

      <Card title="日报生成" className="daily-card">
        <div className="generate-bar">
          <Space>
            <span>日报日期</span>
            <DatePicker value={selectedDate} onChange={(date) => setSelectedDate(date || dayjs())} allowClear={false} />
            <Tag color="green">已选任务：{selectedFolders.length}</Tag>
          </Space>
          <Button type="primary" icon={<FileTextOutlined />} loading={generateLoading} onClick={handleGenerate}>
            生成每条任务日报
          </Button>
        </div>

        {logs.length > 0 && (
          <div className="logs-preview">
            {logs.map((item) => (
              <div className="log-row" key={`${item.folder_path}-${item.title}`}>
                <span className="log-title">{item.title}</span>
                <span className="log-content">{item.content}</span>
              </div>
            ))}
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
            autoSize={{ minRows: 6, maxRows: 14 }}
            placeholder="- 完成了……，计划完成……"
          />
        </div>
      </Card>
    </div>
  )
}

export default DailyReport
