import { Modal, Radio, Switch, Tag } from 'antd'
import { USE_MOCK } from '../../services/api'
import { saveSettings } from '../../services/settings'

function GeneralSettingsSection({ settings, onSettingsChange }) {
  const updateSetting = (key, value) => {
    const newSettings = saveSettings({ [key]: value })
    onSettingsChange(newSettings)
  }

  const handleWindowStyleChange = async (e) => {
    const newStyle = e.target.value
    updateSetting('windowStyle', newStyle)

    if (window.electronAPI?.saveSetting) {
      await window.electronAPI.saveSetting('windowStyle', newStyle)
    }

    Modal.confirm({
      title: '重启生效',
      content: '窗口样式的更改需要重启应用后才会生效，是否立即重启？',
      okText: '立即重启',
      cancelText: '稍后重启',
      onOk: () => {
        if (window.electronAPI?.restartApp) {
          window.electronAPI.restartApp()
        }
      }
    })
  }

  const handlePreviewUpdatesChange = async (checked) => {
    updateSetting('enablePreviewUpdates', checked)

    if (window.electronAPI?.saveSetting) {
      await window.electronAPI.saveSetting('enablePreviewUpdates', checked)
    }
  }

  const handleTitleLanguageChange = (event) => {
    updateSetting('appTitleLanguage', event.target.value)
  }

  return (
    <div id="general-settings" className="settings-block">
      <h2>常规设置</h2>

      <div className="general-settings-list">
        <div className="general-setting-row">
          <div className="general-setting-heading">
            <h3>运行模式</h3>
          </div>
          <div className="general-setting-control mode-status">
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

        <div className="general-setting-row">
          <div className="general-setting-heading">
            <h3>窗口样式</h3>
          </div>
          <div className="general-setting-control">
            <Radio.Group
              className="window-style-control"
              onChange={handleWindowStyleChange}
              value={settings.windowStyle}
              optionType="button"
              buttonStyle="solid"
            >
              <Radio.Button value="integrated">一体化</Radio.Button>
              <Radio.Button value="classic">经典</Radio.Button>
            </Radio.Group>
          </div>
        </div>

        <div className="general-setting-row">
          <div className="general-setting-heading">
            <h3>标题显示</h3>
          </div>
          <div className="general-setting-control">
            <Radio.Group
              className="title-language-control"
              onChange={handleTitleLanguageChange}
              value={settings.appTitleLanguage}
              optionType="button"
              buttonStyle="solid"
            >
              <Radio.Button value="zh">中文（绳结）</Radio.Button>
              <Radio.Button value="en">English（Knot）</Radio.Button>
            </Radio.Group>
            <p className="setting-hint">切换侧边栏与关于页的应用标题。</p>
          </div>
        </div>

        <div className="general-setting-row">
          <div className="general-setting-heading">
            <h3>开发者模式</h3>
            <p className="setting-hint">
              开启后在关于页提供更新详情，并显示仅用于问题排查的诊断信息。
            </p>
          </div>
          <div className="general-setting-control general-setting-control-end">
            <Switch
              checked={!!settings.developerMode}
              onChange={(checked) => updateSetting('developerMode', checked)}
              aria-label="开发者模式"
            />
          </div>
        </div>

        <div className="general-setting-row">
          <div className="general-setting-heading">
            <h3>更新通道</h3>
          </div>
          <div className="general-setting-control">
            <div className="setting-item inline general-preview-update">
              <label>启用预览版更新</label>
              <Switch
                checked={!!settings.enablePreviewUpdates}
                onChange={handlePreviewUpdatesChange}
              />
            </div>
            <p className="setting-hint">
              关闭时只跟踪正式版；开启后会允许检查 alpha/preview 预览版。
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}

export default GeneralSettingsSection
