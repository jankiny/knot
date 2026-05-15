import { useEffect, useMemo, useState } from 'react'
import { Button, Modal, Progress, Space, Typography, message } from 'antd'
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

const isBusyStatus = (status) => ['checking', 'available', 'downloading'].includes(status)

function About() {
  const [feedbackOpen, setFeedbackOpen] = useState(false)
  const [updateStatus, setUpdateStatus] = useState({
    status: 'idle',
    currentVersion: null,
    availableVersion: null,
    progress: null,
    message: ''
  })

  useEffect(() => {
    let mounted = true

    if (window.electronAPI?.getUpdateStatus) {
      window.electronAPI.getUpdateStatus().then((status) => {
        if (mounted && status) {
          setUpdateStatus(status)
        }
      })
    }

    if (window.electronAPI?.onUpdateStatus) {
      window.electronAPI.onUpdateStatus((status) => {
        if (status) {
          setUpdateStatus(status)
        }
      })
    }

    return () => {
      mounted = false
      window.electronAPI?.removeUpdateStatusListener?.()
    }
  }, [])

  const checking = isBusyStatus(updateStatus.status)
  const progressPercent = Math.round(updateStatus.progress?.percent || 0)

  const updateText = useMemo(() => {
    if (!updateStatus.message) return ''
    if (updateStatus.availableVersion) {
      return `${updateStatus.message} v${updateStatus.availableVersion}`
    }
    return updateStatus.message
  }, [updateStatus])

  const handleCopyEmail = async () => {
    try {
      await navigator.clipboard.writeText(FEEDBACK_EMAIL)
      message.success('邮箱已复制')
    } catch (error) {
      message.error('复制失败，请手动复制邮箱')
    }
  }

  const handleCheckUpdate = async () => {
    if (!window.electronAPI?.checkForUpdates) {
      await openExternal(RELEASE_URL)
      message.info('已打开版本发布页面')
      return
    }

    const status = await window.electronAPI.checkForUpdates()
    if (status?.status === 'unsupported') {
      message.info(status.message)
    } else if (status?.status === 'error') {
      message.error(status.message || '检查更新失败')
    }
  }

  const handleInstallUpdate = async () => {
    const result = await window.electronAPI?.installUpdate?.()
    if (!result?.ok) {
      message.error(result?.message || '更新尚未下载完成')
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
          disabled={checking}
          className="about-update-button"
        >
          检查更新
        </Button>

        {updateText && (
          <div className={`about-update-status about-update-status-${updateStatus.status}`}>
            {updateText}
          </div>
        )}

        {updateStatus.status === 'downloading' && (
          <Progress
            className="about-update-progress"
            percent={progressPercent}
            size="small"
          />
        )}

        {updateStatus.status === 'downloaded' && (
          <Button type="primary" onClick={handleInstallUpdate} className="about-install-button">
            重启并安装
          </Button>
        )}

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
            复制邮箱
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
          如果您在使用 Knot 时遇到任何问题或有任何建议，可以通过电子邮件与我们联系。
        </p>
        <p className="about-email">{FEEDBACK_EMAIL}</p>
        <p>也可以在 GitHub 上提交 issue。</p>
      </Modal>
    </div>
  )
}

export default About
