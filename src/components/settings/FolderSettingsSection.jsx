import { useEffect, useState } from 'react'
import { Button, Input, message, Select, Space, Tooltip } from 'antd'
import { FolderOutlined, QuestionCircleOutlined } from '@ant-design/icons'
import { formatFolderName, saveSettings } from '../../services/settings'

const FORMAT_PRESETS = [
  { label: '日期_主题', value: '{{YYYY}}.{{MM}}.{{DD}}_{{subject}}' },
  { label: '日期-主题', value: '{{YYYY}}-{{MM}}-{{DD}}_{{subject}}' },
  { label: '主题_日期', value: '{{subject}}_{{YYYY}}.{{MM}}.{{DD}}' },
  { label: '发件人_日期_主题', value: '{{from}}_{{YYYY}}.{{MM}}.{{DD}}_{{subject}}' },
  { label: '自定义', value: 'custom' }
]

const EXAMPLE_MAIL = {
  subject: '关于项目进度的通知',
  from: '张三 <zhangsan@company.com>',
  date: '2025-01-19 10:30:00'
}

function FolderSettingsSection({ settings, onSettingsChange }) {
  const [formatPreset, setFormatPreset] = useState('preset')

  useEffect(() => {
    const preset = FORMAT_PRESETS.find((item) => item.value === settings.folderNameFormat)
    setFormatPreset(preset ? settings.folderNameFormat : 'custom')
  }, [settings.folderNameFormat])

  const updateSetting = (key, value) => {
    const newSettings = saveSettings({ [key]: value })
    onSettingsChange(newSettings)
  }

  const handleFormatPresetChange = (value) => {
    setFormatPreset(value)
    if (value !== 'custom') {
      updateSetting('folderNameFormat', value)
    }
  }

  const handleSelectFolder = async () => {
    if (window.electronAPI?.selectFolder) {
      const selectedPath = await window.electronAPI.selectFolder()
      if (selectedPath) {
        updateSetting('folderPath', selectedPath)
        message.success('文件夹路径已更新')
      }
    } else {
      message.info('请手动输入文件夹路径，或在 Electron 应用中使用文件夹选择')
    }
  }

  const previewFolderName = formatFolderName(settings.folderNameFormat, EXAMPLE_MAIL)

  return (
    <div id="folder-settings" className="settings-block">
      <h2>文件夹设置</h2>
      <div className="settings-section">
        <div className="section-header">
          <h3>基本设置</h3>
        </div>

        <div className="setting-item">
          <label>
            工作目录
            <Tooltip title="工作目录是新任务文件夹的创建位置，也是自动归档默认扫描的位置。建议选择桌面或一个固定的工作材料目录。">
              <QuestionCircleOutlined className="settings-help-icon" />
            </Tooltip>
          </label>
          <Space.Compact style={{ width: '100%' }}>
            <Input
              prefix={<FolderOutlined />}
              value={settings.folderPath}
              onChange={(e) => updateSetting('folderPath', e.target.value)}
              placeholder="例如: ~/Desktop"
            />
            <Button onClick={handleSelectFolder}>选择</Button>
          </Space.Compact>
        </div>

        <div className="setting-item">
          <label>命名格式</label>
          <Select
            style={{ width: '100%' }}
            value={formatPreset}
            onChange={handleFormatPresetChange}
            options={FORMAT_PRESETS}
          />
        </div>

        {formatPreset === 'custom' && (
          <div className="setting-item">
            <label>自定义格式</label>
            <Input
              value={settings.folderNameFormat}
              onChange={(e) => updateSetting('folderNameFormat', e.target.value)}
              placeholder="{{YYYY}}.{{MM}}.{{DD}}_{{subject}}"
            />
            <p className="setting-hint">
              可用变量：{'{{YYYY}}'} 年、{'{{MM}}'} 月、{'{{DD}}'} 日、{'{{subject}}'} 主题、{'{{from}}'} 发件人
            </p>
          </div>
        )}

        <div className="preview-box">
          <span className="preview-label">预览：</span>
          <span className="preview-value">{previewFolderName}</span>
        </div>
      </div>

      <div className="settings-section" style={{ marginTop: 24 }}>
        <div className="section-header">
          <h3>标准任务结构</h3>
        </div>
        <p className="setting-hint">
          实际目录由创建任务时选择的 SOP 模板决定。以下是“通用任务”的默认结构；学习笔记、照片项目等模板会使用自己的目录。
        </p>
        <div className="folder-structure-preview">
          <p className="preview-label">通用任务目录预览：</p>
          <div className="tree">
            <div className="tree-item">{previewFolderName}/</div>
            <div className="tree-item level-1">00_来源资料/</div>
            <div className="tree-item level-2">email.txt（仅邮件任务）</div>
            <div className="tree-item level-2">email.pdf（仅邮件任务）</div>
            <div className="tree-item level-2">附件/（仅邮件任务）</div>
            <div className="tree-item level-1">10_过程文件/</div>
            <div className="tree-item level-1">20_成果输出/</div>
            <div className="tree-item level-1">工作记录.md</div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default FolderSettingsSection
