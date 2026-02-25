import { useState, useEffect } from 'react'
import { Drawer, Form, Input, InputNumber, Button, Switch, message, Divider, Tag, Space, Select, Checkbox, Anchor } from 'antd'
import { MailOutlined, LockOutlined, GlobalOutlined, FolderOutlined } from '@ant-design/icons'
import { mailApi, USE_MOCK } from '../services/api'
import { getSettings, saveSettings, formatFolderName } from '../services/settings'
import DepartmentManager from './DepartmentManager'
import './Settings.css'

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

function Settings() {
  const [loading, setLoading] = useState(false)
  const [connected, setConnected] = useState(false)
  const [settings, setSettings] = useState(getSettings())
  const [formatPreset, setFormatPreset] = useState('preset')
  const [form] = Form.useForm()

  useEffect(() => {
    const loadSettings = async () => {
      const s = getSettings()
      setSettings(s)
      // 判断当前格式是否为预设
      const preset = FORMAT_PRESETS.find(p => p.value === s.folderNameFormat)
      setFormatPreset(preset ? s.folderNameFormat : 'custom')

      // 解密密码
      let decryptedPassword = ''
      if (s.mailPasswordEncrypted && window.electronAPI?.decryptPassword) {
        try {
          decryptedPassword = await window.electronAPI.decryptPassword(s.mailPasswordEncrypted) || ''
        } catch (e) {
          console.error('解密密码失败:', e)
        }
      }

      // 加载邮件服务器配置到表单
      form.setFieldsValue({
        server: s.mailServer || '',
        port: s.mailPort || 993,
        username: s.mailUsername || '',
        password: decryptedPassword,
        use_ssl: s.mailUseSsl !== false
      })
    }

    loadSettings()
  }, [form])

  const handleConnect = async (values) => {
    setLoading(true)
    try {
      await mailApi.connect({
        server: values.server,
        port: values.port,
        username: values.username,
        password: values.password,
        use_ssl: values.use_ssl
      })
      message.success('连接成功')
      setConnected(true)

      // 加密密码后保存
      let encryptedPassword = null
      if (values.password && window.electronAPI?.encryptPassword) {
        try {
          encryptedPassword = await window.electronAPI.encryptPassword(values.password)
        } catch (e) {
          console.error('加密密码失败:', e)
        }
      }

      // 保存邮件服务器配置（密码加密存储）
      saveSettings({
        mailServer: values.server,
        mailPort: values.port,
        mailUsername: values.username,
        mailPasswordEncrypted: encryptedPassword,  // 存储加密后的密码
        mailUseSsl: values.use_ssl
      })
    } catch (error) {
      // 更详细的错误信息
      let errorMsg = '连接失败'
      if (error.code === 'ERR_NETWORK') {
        errorMsg = '无法连接到后端服务，请检查后端是否正常启动'
      } else if (error.response?.data?.detail) {
        errorMsg = error.response.data.detail
      } else if (error.message) {
        errorMsg = error.message
      }
      message.error(errorMsg)
      console.error('邮件连接错误:', error)
    } finally {
      setLoading(false)
    }
  }

  const updateSetting = (key, value) => {
    const newSettings = saveSettings({ [key]: value })
    setSettings(newSettings)
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

  // 预览文件夹名称
  const previewFolderName = formatFolderName(settings.folderNameFormat, EXAMPLE_MAIL)

  return (
    <div className="settings-container">
      <div className="settings-content">
        {/* 常规设置 */}
        <div id="general-settings" className="settings-block">
          <h2>常规设置</h2>
          <div className="settings-section">
            <div className="section-header">
              <h3>运行模式</h3>
            </div>
            <div className="mode-status">
              {USE_MOCK ? (
                <Tag color="orange">Mock 模式（外网开发）</Tag>
              ) : (
                <Tag color="green">已连接邮件服务器</Tag>
              )}
              <p className="mode-hint">
                {USE_MOCK
                  ? 'Mock 模式下使用模拟邮件数据，但文件夹创建为真实操作'
                  : '当前连接真实邮件服务器'}
              </p>
            </div>
          </div>
        </div>

        <Divider />

        {/* 邮件设置 */}
        <div id="mail-settings" className="settings-block">
          <h2>邮件设置</h2>
          <div className="settings-section">
            <div className="section-header">
              <h3>邮件服务器配置</h3>
              {connected && <Tag color="success">已连接</Tag>}
            </div>

            <div className="setting-item">
              <label>获取数量限制</label>
              <InputNumber
                min={1}
                max={1000}
                style={{ width: '100%' }}
                value={settings.mailLimit || 50}
                onChange={(val) => updateSetting('mailLimit', val)}
              />
              <p className="setting-hint">每次获取的最新邮件数量</p>
            </div>

            <div className="setting-item">
              <label>时间范围 (天)</label>
              <InputNumber
                min={0}
                max={365}
                style={{ width: '100%' }}
                value={settings.mailDays !== undefined ? settings.mailDays : 7}
                onChange={(val) => updateSetting('mailDays', val)}
              />
              <p className="setting-hint">仅获取最近几天的邮件 (0表示不限制)</p>
            </div>
            <Divider style={{ margin: '12px 0' }} />

            <Form
              form={form}
              layout="vertical"
              onFinish={handleConnect}
              initialValues={{
                port: 993,
                use_ssl: true
              }}
              disabled={USE_MOCK}
            >
              <Form.Item
                name="server"
                label="服务器地址"
                rules={[{ required: !USE_MOCK, message: '请输入服务器地址' }]}
              >
                <Input prefix={<GlobalOutlined />} placeholder="例如: mail.example.com" />
              </Form.Item>

              <Form.Item
                name="port"
                label="端口"
                rules={[{ required: !USE_MOCK, message: '请输入端口' }]}
              >
                <InputNumber min={1} max={65535} style={{ width: '100%' }} />
              </Form.Item>

              <Form.Item
                name="username"
                label="用户名/邮箱"
                rules={[{ required: !USE_MOCK, message: '请输入用户名' }]}
              >
                <Input prefix={<MailOutlined />} placeholder="你的邮箱地址" />
              </Form.Item>

              <Form.Item
                name="password"
                label="密码"
                rules={[{ required: !USE_MOCK, message: '请输入密码' }]}
              >
                <Input.Password prefix={<LockOutlined />} placeholder="邮箱密码" />
              </Form.Item>

              <Form.Item
                name="use_ssl"
                label="使用 SSL"
                valuePropName="checked"
              >
                <Switch />
              </Form.Item>

              <Form.Item>
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={loading}
                  block
                  disabled={USE_MOCK}
                >
                  {USE_MOCK ? 'Mock 模式下无需连接' : '连接邮箱'}
                </Button>
              </Form.Item>
            </Form>
          </div>
        </div>

        <Divider />

        {/* 文件夹设置 */}
        <div id="folder-settings" className="settings-block">
          <h2>文件夹设置</h2>
          <div className="settings-section">
            <div className="section-header">
              <h3>基本设置</h3>
            </div>

            <div className="setting-item">
              <label>保存位置</label>
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
                  {settings.saveMailContent && (settings.saveFormats || ['txt']).map(fmt => (
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
                  {settings.saveMailContent && (settings.saveFormats || ['txt']).map(fmt => (
                    <div key={fmt} className="tree-item level-1">{settings.mailContentFileName}.{fmt}</div>
                  ))}
                  <div className="tree-item level-1">附件1.pdf</div>
                  <div className="tree-item level-1">附件2.docx</div>
                </div>
              </div>
            )}
          </div>
        </div>

        <Divider />

        {/* 归档设置 */}
        <div id="archive-settings" className="settings-block">
          <h2>归档设置</h2>
          <div className="settings-section">
            <div className="section-header">
              <h3>部门管理</h3>
            </div>
            <p className="setting-hint" style={{ marginBottom: 12 }}>
              管理部门及其归档路径，用于自动归档功能
            </p>
            <DepartmentManager />
          </div>
        </div>
      </div>

      {/* 右侧导航 */}
      <div className="settings-nav">
        <Anchor
          offsetTop={24}
          items={[
            {
              key: 'general-settings',
              href: '#general-settings',
              title: '常规设置',
            },
            {
              key: 'mail-settings',
              href: '#mail-settings',
              title: '邮件设置',
            },
            {
              key: 'folder-settings',
              href: '#folder-settings',
              title: '文件夹设置',
            },
            {
              key: 'archive-settings',
              href: '#archive-settings',
              title: '归档设置',
            }
          ]}
        />
      </div>
    </div>
  )
}

export default Settings
