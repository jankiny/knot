import { useState } from 'react'
import { Button, Modal, Space, Typography, message } from 'antd'
import {
  CopyOutlined,
  GithubOutlined,
  MailOutlined,
  ReloadOutlined
} from '@ant-design/icons'
import appIcon from '../../electron/assets/icons/512x512.png'
import './About.css'

const { Text } = Typography

const PROJECT_URL = 'https://github.com/jankiny/knot'
const RELEASE_URL = 'https://github.com/jankiny/knot/releases'
const FEEDBACK_EMAIL = 'yangzlnn@foxmail.com'

const getVersion = () => {
  if (typeof window !== 'undefined' && window.electronAPI?.version) {
    return `v${window.electronAPI.version}`
  }
  return 'dev'
}

const openExternal = async (url) => {
  if (window.electronAPI?.openExternal) {
    await window.electronAPI.openExternal(url)
    return
  }
  window.open(url, '_blank', 'noopener,noreferrer')
}

function About() {
  const [feedbackOpen, setFeedbackOpen] = useState(false)
  const [checking, setChecking] = useState(false)

  const handleCopyEmail = async () => {
    try {
      await navigator.clipboard.writeText(FEEDBACK_EMAIL)
      message.success('邮箱已复制')
    } catch (error) {
      message.error('复制失败，请手动复制邮箱')
    }
  }

  const handleCheckUpdate = async () => {
    setChecking(true)
    try {
      await openExternal(RELEASE_URL)
      message.info('已打开版本发布页面')
    } finally {
      setChecking(false)
    }
  }

  return (
    <div className="about-page">
      <div className="about-panel">
        <img
          className="about-logo"
          src={appIcon}
          alt="Knot"
          onError={(event) => {
            event.currentTarget.style.display = 'none'
          }}
        />
        <h1>Knot</h1>
        <Text type="secondary" className="about-version">
          当前版本 {getVersion()}
        </Text>

        <Button
          type="primary"
          size="large"
          icon={<ReloadOutlined />}
          loading={checking}
          onClick={handleCheckUpdate}
          className="about-update-button"
        >
          检查更新
        </Button>

        <Space className="about-actions">
          <Button icon={<GithubOutlined />} onClick={() => openExternal(PROJECT_URL)}>
            项目说明
          </Button>
          <Button icon={<MailOutlined />} onClick={() => setFeedbackOpen(true)}>
            意见反馈
          </Button>
        </Space>

        <div className="about-copyright">版权所有 2026 Knot</div>
      </div>

      <Modal
        title="意见反馈"
        open={feedbackOpen}
        onCancel={() => setFeedbackOpen(false)}
        footer={[
          <Button key="copy" icon={<CopyOutlined />} onClick={handleCopyEmail}>
            复制邮件
          </Button>,
          <Button key="code" icon={<GithubOutlined />} onClick={() => openExternal(PROJECT_URL)}>
            查看代码
          </Button>,
          <Button key="cancel" onClick={() => setFeedbackOpen(false)}>
            取消
          </Button>
        ]}
      >
        <p>
          如果您在使用 Knot 时遇到任何问题或有任何建议，您可以通过电子邮件与我们联系。
        </p>
        <p className="about-email">{FEEDBACK_EMAIL}</p>
        <p>或者在 GitHub 上提交 issue。</p>
      </Modal>
    </div>
  )
}

export default About
