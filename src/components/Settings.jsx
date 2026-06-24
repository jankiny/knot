import { useState } from 'react'
import { Anchor, Divider } from 'antd'
import { getSettings } from '../services/settings'
import AiSettingsSection from './settings/AiSettingsSection'
import ArchiveSettingsSection from './settings/ArchiveSettingsSection'
import FolderSettingsSection from './settings/FolderSettingsSection'
import GeneralSettingsSection from './settings/GeneralSettingsSection'
import MailSettingsSection from './settings/MailSettingsSection'
import SopSettingsSection from './settings/SopSettingsSection'
import './Settings.css'

function Settings() {
  const [settings, setSettings] = useState(getSettings())

  return (
    <div className="settings-container">
      <div className="settings-content">
        <GeneralSettingsSection settings={settings} onSettingsChange={setSettings} />

        <Divider />

        <MailSettingsSection settings={settings} onSettingsChange={setSettings} />

        <Divider />

        <FolderSettingsSection settings={settings} onSettingsChange={setSettings} />

        <Divider />

        <SopSettingsSection settings={settings} onSettingsChange={setSettings} />

        <Divider />

        <AiSettingsSection settings={settings} onSettingsChange={setSettings} />

        <Divider />

        <ArchiveSettingsSection onSettingsChange={setSettings} />
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
              key: 'sop-settings',
              href: '#sop-settings',
              title: 'SOP 模板',
            },
            {
              key: 'ai-settings',
              href: '#ai-settings',
              title: 'AI 设置',
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
