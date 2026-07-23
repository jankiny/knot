import { useEffect, useState } from 'react'
import { Button, Modal, Progress, Space, Typography, message } from 'antd'
import {
  CopyOutlined,
  DownloadOutlined,
  GithubOutlined,
  InfoCircleOutlined,
  MailOutlined,
  ReloadOutlined
} from '@ant-design/icons'
import appIcon from '../../electron/assets/icons/512x512.png'
import './About.css'

const { Text } = Typography

const PROJECT_URL = 'https://github.com/jankiny/knot'
const RELEASE_URL = 'https://github.com/jankiny/knot/releases'
const FEEDBACK_EMAIL = 'yangzlnn@foxmail.com'

const UPDATE_STATUS_TEXT = {
  idle: '未检查',
  checking: '正在检查',
  'not-available': '已是最新',
  available: '发现新版本',
  downloading: '正在下载',
  downloaded: '下载完成',
  manual: '需要手动更新',
  'incomplete-release': '发布包不完整',
  'network-unavailable': '网络不可用',
  unsupported: '当前环境不支持自动更新',
  'error-handled': '已降级处理'
}

const openExternal = async (url) => {
  if (window.electronAPI?.openExternal) {
    await window.electronAPI.openExternal(url)
    return
  }
  window.open(url, '_blank', 'noopener,noreferrer')
}

const isBusyStatus = (status) => ['checking', 'available', 'downloading'].includes(status)
const requiresManualUpdate = (status) => [
  'manual',
  'incomplete-release',
  'network-unavailable',
  'unsupported',
  'error-handled'
].includes(status)

const formatVersion = (version) => {
  if (!version) return '未获取到'
  const text = String(version)
  return text.startsWith('v') ? text : `v${text}`
}

function About({ appTitle = 'Knot', developerMode = false }) {
  const [feedbackOpen, setFeedbackOpen] = useState(false)
  const [updateDetailsOpen, setUpdateDetailsOpen] = useState(false)
  const [updateStatus, setUpdateStatus] = useState({
    status: 'idle',
    currentVersion: null,
    availableVersion: null,
    progress: null,
    previewUpdates: false,
    releaseUrl: RELEASE_URL,
    manualDownloadUrl: null,
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

    let unsubscribeUpdateStatus
    if (window.electronAPI?.onUpdateStatus) {
      unsubscribeUpdateStatus = window.electronAPI.onUpdateStatus((status) => {
        if (status) {
          setUpdateStatus(status)
        }
      })
    }

    return () => {
      mounted = false
      if (typeof unsubscribeUpdateStatus === 'function') {
        unsubscribeUpdateStatus()
      }
    }
  }, [])

  const checking = isBusyStatus(updateStatus.status)
  const progressPercent = Math.round(updateStatus.progress?.percent || 0)
  const updateChannel = updateStatus.channelLabel || (updateStatus.previewUpdates ? '预览版' : '正式版')
  const currentVersion = formatVersion(updateStatus.currentVersion || window.electronAPI?.version)
  const latestVersion = formatVersion(updateStatus.latestVersion || updateStatus.availableVersion)
  const updateStatusLabel = UPDATE_STATUS_TEXT[updateStatus.status] || UPDATE_STATUS_TEXT['error-handled']
  const updateText = updateStatus.message || ''
  const hasDiagnostics = Boolean(
    updateStatus.diagnostics && Object.keys(updateStatus.diagnostics).length > 0
  )
  const hasUpdateDetails = updateStatus.status !== 'idle' || Boolean(updateText) || hasDiagnostics

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
    } else if (requiresManualUpdate(status?.status)) {
      message.info(status.message || '已提供手动更新入口')
    }
  }

  const handleManualDownload = async () => {
    await openExternal(updateStatus.manualDownloadUrl || updateStatus.releaseUrl || RELEASE_URL)
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
        <div className="about-brand">
          <img
            className="about-logo"
            src={appIcon}
            alt={appTitle}
            onError={(event) => {
              event.currentTarget.style.display = 'none'
            }}
          />
          <div className="about-brand-copy">
            <h1>{appTitle}</h1>
            <Text type="secondary" className="about-version">
              当前版本 {currentVersion}
            </Text>
          </div>
        </div>

        <section className="about-update-card" aria-label="版本更新">
          <div className="about-update-details">
            <div>
              <span>监测通道</span>
              <strong title={updateChannel}>{updateChannel}</strong>
            </div>
            <div>
              <span>最新版本</span>
              <strong title={latestVersion}>{latestVersion}</strong>
            </div>
            <div>
              <span>更新状态</span>
              <strong title={updateStatusLabel}>{updateStatusLabel}</strong>
            </div>
          </div>

          <div
            className={`about-update-feedback about-update-feedback-${updateStatus.status}`}
            role="status"
            aria-live="polite"
          >
            {updateStatus.status === 'downloading' ? (
              <Progress percent={progressPercent} size="small" />
            ) : (
              <span title={updateText || undefined}>
                {updateText || '点击“检查更新”获取最新版本信息'}
              </span>
            )}
          </div>

          <div className="about-update-controls">
            <Button
              type="primary"
              icon={<ReloadOutlined />}
              loading={checking}
              onClick={handleCheckUpdate}
              disabled={checking}
              className="about-update-button"
            >
              检查更新
            </Button>

            {updateStatus.status === 'downloaded' ? (
              <div className="about-update-primary-action">
                <Button type="primary" onClick={handleInstallUpdate} block>
                  重启并安装
                </Button>
              </div>
            ) : requiresManualUpdate(updateStatus.status) ? (
              <div className="about-update-primary-action">
                <Button icon={<DownloadOutlined />} onClick={handleManualDownload} block>
                  打开发布页
                </Button>
              </div>
            ) : null}

            {developerMode ? (
              <Button
                icon={<InfoCircleOutlined />}
                onClick={() => setUpdateDetailsOpen(true)}
                disabled={!hasUpdateDetails}
                className="about-update-details-button"
              >
                更新详情
              </Button>
            ) : null}
          </div>
        </section>

        <Space className="about-actions">
          <Button icon={<DownloadOutlined />} onClick={() => openExternal(updateStatus.releaseUrl || RELEASE_URL)}>
            发布页
          </Button>
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
        title="更新详情"
        open={developerMode && updateDetailsOpen}
        centered
        onCancel={() => setUpdateDetailsOpen(false)}
        footer={[
          updateStatus.status === 'downloaded' ? (
            <Button key="install" type="primary" onClick={handleInstallUpdate}>
              重启并安装
            </Button>
          ) : null,
          requiresManualUpdate(updateStatus.status) ? (
            <Button key="release" type="primary" icon={<DownloadOutlined />} onClick={handleManualDownload}>
              打开发布页
            </Button>
          ) : null,
          <Button key="close" onClick={() => setUpdateDetailsOpen(false)}>
            关闭
          </Button>
        ].filter(Boolean)}
      >
        <div className="about-update-modal">
          <div className="about-update-modal-summary">
            <div>
              <span>当前版本</span>
              <strong>{currentVersion}</strong>
            </div>
            <div>
              <span>最新版本</span>
              <strong>{latestVersion}</strong>
            </div>
            <div>
              <span>更新状态</span>
              <strong>{updateStatusLabel}</strong>
            </div>
          </div>

          <div className="about-update-modal-message">
            {updateText || '暂无更多更新说明。'}
          </div>

          {updateStatus.status === 'downloading' ? (
            <Progress percent={progressPercent} />
          ) : null}

          {developerMode && hasDiagnostics ? (
            <div className="about-update-diagnostics">
              <div className="about-update-diagnostics-title">更新诊断信息</div>
              <pre>{JSON.stringify(updateStatus.diagnostics, null, 2)}</pre>
            </div>
          ) : null}
        </div>
      </Modal>

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
