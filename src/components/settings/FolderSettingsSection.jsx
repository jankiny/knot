import { useEffect, useState } from 'react'
import { Button, Checkbox, Input, message, Select, Space, Switch, Tooltip } from 'antd'
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

const SAVE_FORMAT_OPTIONS = [
  { label: 'TXT（纯文本）', value: 'txt' },
  { label: 'EML（邮件原始格式）', value: 'eml' },
  { label: 'PDF（便于打印）', value: 'pdf' }
]

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
          <h3>内容组织</h3>
        </div>

        <div className="setting-item inline">
          <label>添加子目录</label>
          <Switch
            checked={settings.useSubFolder}
            onChange={(checked) => updateSetting('useSubFolder', checked)}
          />
        </div>
        <p className="setting-hint">
          开启后，邮件内容和附件将保存到子目录中
        </p>

        {settings.useSubFolder && (
          <div className="setting-item" style={{ marginTop: 12 }}>
            <label>子目录名称</label>
            <Input
              value={settings.subFolderName}
              onChange={(e) => updateSetting('subFolderName', e.target.value)}
              placeholder="邮件"
            />
          </div>
        )}

        <div className="setting-item inline" style={{ marginTop: 16 }}>
          <label>保存邮件正文</label>
          <Switch
            checked={settings.saveMailContent}
            onChange={(checked) => updateSetting('saveMailContent', checked)}
          />
        </div>

        {settings.saveMailContent && (
          <div className="setting-item" style={{ marginTop: 12 }}>
            <label>正文文件名</label>
            <Input
              value={settings.mailContentFileName}
              onChange={(e) => updateSetting('mailContentFileName', e.target.value)}
              placeholder="邮件正文"
            />
            <p className="setting-hint">不含扩展名，扩展名由保存格式决定</p>
          </div>
        )}

        {settings.saveMailContent && (
          <div className="setting-item">
            <label>保存格式</label>
            <Checkbox.Group
              options={SAVE_FORMAT_OPTIONS}
              value={settings.saveFormats || ['txt']}
              onChange={(checkedValues) => {
                if (checkedValues.length === 0) {
                  message.warning('请至少选择一种保存格式')
                  return
                }
                updateSetting('saveFormats', checkedValues)
              }}
              style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}
            />
            <p className="setting-hint">
              TXT：纯文本格式，兼容性最好；EML：邮件原始格式，可用邮件客户端打开；PDF：便于打印和分享
            </p>
          </div>
        )}

        {settings.useSubFolder && (
          <div className="folder-structure-preview">
            <p className="preview-label">目录结构预览：</p>
            <div className="tree">
              <div className="tree-item">{previewFolderName}/</div>
              <div className="tree-item level-1">{settings.subFolderName}/</div>
              {settings.saveMailContent && (settings.saveFormats || ['txt']).map((fmt) => (
                <div key={fmt} className="tree-item level-2">{settings.mailContentFileName}.{fmt}</div>
              ))}
              <div className="tree-item level-2">附件1.pdf</div>
              <div className="tree-item level-2">附件2.docx</div>
            </div>
          </div>
        )}

        {!settings.useSubFolder && (
          <div className="folder-structure-preview">
            <p className="preview-label">目录结构预览：</p>
            <div className="tree">
              <div className="tree-item">{previewFolderName}/</div>
              {settings.saveMailContent && (settings.saveFormats || ['txt']).map((fmt) => (
                <div key={fmt} className="tree-item level-1">{settings.mailContentFileName}.{fmt}</div>
              ))}
              <div className="tree-item level-1">附件1.pdf</div>
              <div className="tree-item level-1">附件2.docx</div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

export default FolderSettingsSection
