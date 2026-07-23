import { useEffect, useState } from 'react'
import { Button, message, Select, Space, Tag } from 'antd'
import { FolderOutlined } from '@ant-design/icons'
import { sopApi } from '../../services/api'
import { getSettings, setDefaultSopTemplateId } from '../../services/settings'

function SopSettingsSection({ settings, onSettingsChange }) {
  const [sopTemplates, setSopTemplates] = useState([])
  const [sopRoots, setSopRoots] = useState([])

  const loadSopTemplates = async () => {
    try {
      const result = await sopApi.listTemplates()
      setSopTemplates(result.templates || [])
      setSopRoots(result.roots || [])
    } catch (error) {
      message.error('加载 SOP 模板失败')
    }
  }

  useEffect(() => {
    loadSopTemplates()
  }, [])

  const handleDefaultSopChange = (value) => {
    setDefaultSopTemplateId(value)
    onSettingsChange(getSettings())
    message.success('默认 SOP 模板已更新')
  }

  const handleOpenSopFolder = async () => {
    const targetPath = sopRoots[0]
    if (!targetPath) {
      message.warning('暂无可打开的模板目录，请先刷新模板列表')
      return
    }
    if (!window.electronAPI?.openFolder) {
      message.info('请在 Electron 客户端中打开模板目录')
      return
    }
    const ok = await window.electronAPI.openFolder(targetPath)
    if (!ok) {
      message.error('打开模板目录失败')
    }
  }

  const templates = sopTemplates.length
    ? sopTemplates
    : [{ id: 'default-task', name: '通用任务', builtin: true }]

  return (
    <div id="sop-settings" className="settings-block">
      <h2>SOP 模板</h2>
      <div className="settings-section">
        <div className="section-header">
          <h3>标准化流程模板</h3>
          <Space>
            <Button size="small" icon={<FolderOutlined />} onClick={handleOpenSopFolder}>
              打开模板文件夹
            </Button>
            <Button size="small" onClick={loadSopTemplates}>刷新</Button>
          </Space>
        </div>

        <div className="setting-item">
          <label>默认模板</label>
          <Select
            style={{ width: '100%' }}
            value={settings.defaultSopTemplateId || 'default-task'}
            onChange={handleDefaultSopChange}
            options={templates.map((tpl) => ({
              label: tpl.name,
              value: tpl.id
            }))}
          />
          <p className="setting-hint">邮件生成和快速创建时会默认选中这个标准流程。</p>
        </div>

        <div className="folder-structure-preview">
          <p className="preview-label">已识别模板：</p>
          <div className="tree">
            {templates.map((tpl) => (
              <div className="tree-item" key={tpl.id}>
                {tpl.name}
                <Tag color={tpl.builtin ? 'blue' : 'green'} style={{ marginLeft: 8 }}>
                  {tpl.builtin ? '内置' : '已安装'}
                </Tag>
              </div>
            ))}
          </div>
        </div>

        <div className="folder-structure-preview">
          <p className="preview-label">模板安装目录：</p>
          <div className="tree">
            {sopRoots.map((root) => (
              <div className="tree-item" key={root}>{root}</div>
            ))}
          </div>
          <p className="setting-hint">
            分享的 SOP 包解压到上述 sop-templates 目录后，点击刷新即可识别。
          </p>
        </div>
      </div>
    </div>
  )
}

export default SopSettingsSection
