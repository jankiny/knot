import { useEffect, useState } from 'react'
import { Button, Divider, Form, Input, InputNumber, message, Switch, Tag } from 'antd'
import { GlobalOutlined, LockOutlined, MailOutlined } from '@ant-design/icons'
import { mailApi, USE_MOCK } from '../../services/api'
import { saveSettings } from '../../services/settings'

function MailSettingsSection({ settings, onSettingsChange }) {
  const [loading, setLoading] = useState(false)
  const [connected, setConnected] = useState(false)
  const [form] = Form.useForm()

  useEffect(() => {
    const loadMailSettings = async () => {
      let decryptedPassword = ''
      if (settings.mailPasswordEncrypted && window.electronAPI?.decryptPassword) {
        try {
          decryptedPassword = await window.electronAPI.decryptPassword(settings.mailPasswordEncrypted) || ''
        } catch (e) {
          console.error('解密密码失败:', e)
        }
      }

      form.setFieldsValue({
        server: settings.mailServer || '',
        port: settings.mailPort || 993,
        username: settings.mailUsername || '',
        password: decryptedPassword,
        use_ssl: settings.mailUseSsl !== false
      })
    }

    loadMailSettings()
  }, [form, settings])

  const updateSetting = (key, value) => {
    const newSettings = saveSettings({ [key]: value })
    onSettingsChange(newSettings)
  }

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

      let encryptedPassword = null
      if (values.password && window.electronAPI?.encryptPassword) {
        try {
          encryptedPassword = await window.electronAPI.encryptPassword(values.password)
        } catch (e) {
          console.error('加密密码失败:', e)
        }
      }

      const newSettings = saveSettings({
        mailServer: values.server,
        mailPort: values.port,
        mailUsername: values.username,
        mailPasswordEncrypted: encryptedPassword,
        mailUseSsl: values.use_ssl
      })
      onSettingsChange(newSettings)
    } catch (error) {
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

  return (
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
  )
}

export default MailSettingsSection
