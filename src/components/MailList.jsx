import { useState, useEffect, useMemo, useRef } from 'react'
import { List, Button, Tag, message, Spin, Empty, Tooltip, Modal, Alert } from 'antd'
import { ReloadOutlined, SettingOutlined } from '@ant-design/icons'
import { mailApi, folderApi, USE_MOCK } from '../services/api'
import { getSettings, formatFolderName, cleanSubjectForFolder, generateMailHash, getDepartments, getProjects } from '../services/settings'
import { useMailData } from '../hooks/useMailData'
import { useMailGenerationStatus } from '../hooks/useMailGenerationStatus'
import DepartmentSelectModal from './DepartmentSelectModal'
import MailItemCard from './MailItemCard'
import MailPreviewModal from './MailPreviewModal'
import { buildMailTimeline, formatRefreshTime, getMailMonthKey } from './mailListUtils'
import './MailList.css'

function MailList() {
  const [showRefreshTip, setShowRefreshTip] = useState(false)
  const [creating, setCreating] = useState({})
  const [previewMail, setPreviewMail] = useState(null)
  const [loadingDetail, setLoadingDetail] = useState(false)
  // 当前处于视口视野内的主要月份
  const [activeMonthKey, setActiveMonthKey] = useState('')
  // 部门选择弹窗状态
  const [deptModalOpen, setDeptModalOpen] = useState(false)
  const [selectedMailForFolder, setSelectedMailForFolder] = useState(null)
  const refreshTipTimerRef = useRef(null)
  const {
    connectionError,
    fetchDays,
    fetchMails,
    lastRefreshAt,
    loading,
    mails,
    refreshing,
    setLastRefreshAt,
    setMails
  } = useMailData()
  const { getGeneratedStatus, markGenerated } = useMailGenerationStatus(mails)

  const showRefreshHint = (timestamp) => {
    if (!timestamp) return
    setLastRefreshAt(timestamp)
    setShowRefreshTip(true)
    if (refreshTipTimerRef.current) {
      clearTimeout(refreshTipTimerRef.current)
    }
    refreshTipTimerRef.current = setTimeout(() => {
      setShowRefreshTip(false)
    }, 2600)
  }

  useEffect(() => {
    return () => {
      if (refreshTipTimerRef.current) {
        clearTimeout(refreshTipTimerRef.current)
      }
    }
  }, [])

  const timelineData = useMemo(() => buildMailTimeline(mails), [mails])

  // 监听滚动来计算当前所在的月份
  useEffect(() => {
    if (!mails || mails.length === 0 || timelineData.length === 0) return

    let visibleMails = new Map()

    const observer = new IntersectionObserver((entries) => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          visibleMails.set(entry.target.id, entry.boundingClientRect.top)
        } else {
          visibleMails.delete(entry.target.id)
        }
      })

      if (visibleMails.size > 0) {
        // 找到最上面（top最小且最接近0，或者正数最小）的邮件
        let closestId = ''
        let minTop = Infinity

        for (const [id, top] of visibleMails.entries()) {
          // 加上一定的 header 偏移量容差
          const adjustedTop = top - 80
          if (adjustedTop >= 0 && adjustedTop < minTop) {
            minTop = adjustedTop
            closestId = id
          }
        }

        // 如果没有正数的（比如一个极长邮件占满了整个屏幕）
        if (!closestId) {
          let maxTop = -Infinity
          for (const [id, top] of visibleMails.entries()) {
            if (top > maxTop) {
              maxTop = top
              closestId = id
            }
          }
        }

        if (closestId) {
          const actualMailId = closestId.replace('mail-', '')
          const mail = mails.find(m => String(m.id) === String(actualMailId))
          if (mail && mail.date) {
            const monthKey = getMailMonthKey(mail.date)
            setActiveMonthKey(monthKey)
          }
        }
      }
    }, {
      rootMargin: '-80px 0px 0px 0px',
      // 定义多个阈值以便更好地捕获不同大小元素的交叉状态
      threshold: [0, 0.1, 0.5, 0.9, 1]
    })

    mails.forEach(mail => {
      const el = document.getElementById(`mail-${mail.id}`)
      if (el) observer.observe(el)
    })

    return () => observer.disconnect()
  }, [mails, timelineData])

  // 滚动到指定邮件
  const scrollToMail = (mailId) => {
    const element = document.getElementById(`mail-${mailId}`)
    if (element) {
      // 考虑到可能有顶部导航栏，可以设置一个偏移
      const headerOffset = 80 // 假设大概80px
      const elementPosition = element.getBoundingClientRect().top
      const offsetPosition = elementPosition + window.pageYOffset - headerOffset

      // 注意：这里的滚动取决于外层容器是谁
      // 如果是用原生的或者自定义的滚动容器，可能需要用 element.scrollIntoView()
      element.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }

  // 打开部门选择弹窗（先检查重复，再加载邮件详情）
  const openDeptModal = async (mail) => {
    // 检查是否已生成过
    try {
      const mailHash = await generateMailHash(mail)
      const settings = getSettings()
      const departments = getDepartments()
      const projects = getProjects()
      const archivePaths = [...departments, ...projects].map(item => item.archivePath).filter(Boolean)

      const checkResult = await folderApi.checkHash(
        mailHash,
        settings.scanPath || settings.folderPath,
        archivePaths
      )

      if (checkResult.found) {
        const match = checkResult.matches[0]
        const statusText = match.status === 'archived' ? '已归档' : '工作中'
        const confirmed = await new Promise(resolve => {
          Modal.confirm({
            title: '该邮件已生成过工作目录',
            content: (
              <div>
                <p>已存在目录：<strong>{match.name}</strong>（{statusText}）</p>
                <p>再次生成将创建新的工作目录，原有工作记录不受影响。是否继续？</p>
              </div>
            ),
            okText: '继续生成',
            cancelText: '取消',
            onOk: () => resolve(true),
            onCancel: () => resolve(false)
          })
        })
        if (!confirmed) return
      }
    } catch (err) {
      console.error('查重失败:', err)
      // 查重失败不阻断流程
    }

    // 如果邮件还没有正文，先加载详情
    if (!mail.body) {
      try {
        setCreating(prev => ({ ...prev, [mail.id]: true }))
        const result = await mailApi.getMailDetail(mail.id)
        if (result.success && result.data) {
          // 更新邮件列表中的这封邮件
          setMails(prevMails => prevMails.map(m =>
            m.id === mail.id
              ? { ...m, body: result.data.body, attachments: result.data.attachments, raw_content: result.data.raw_content }
              : m
          ))
          mail = { ...mail, body: result.data.body, attachments: result.data.attachments, raw_content: result.data.raw_content }
        }
      } catch (error) {
        console.error('加载邮件详情失败:', error)
      } finally {
        setCreating(prev => ({ ...prev, [mail.id]: false }))
      }
    }
    setSelectedMailForFolder(mail)
    setDeptModalOpen(true)
  }

  // 确认选择归属后创建文件夹
  const handleDeptConfirm = async (target, sopTemplateId) => {
    setDeptModalOpen(false)
    if (selectedMailForFolder) {
      await handleCreateFolder(selectedMailForFolder, target, sopTemplateId)
    }
    setSelectedMailForFolder(null)
  }

  // 创建文件夹（始终包含附件下载）
  const handleCreateFolder = async (mail, target = null, sopTemplateId = 'default-task') => {
    setCreating(prev => ({ ...prev, [mail.id]: true }))
    try {
      // 如果邮件没有正文，先加载详情
      let mailData = mail
      if (!mail.body) {
        try {
          const result = await mailApi.getMailDetail(mail.id)
          if (result.success && result.data) {
            mailData = { ...mail, body: result.data.body, attachments: result.data.attachments, raw_content: result.data.raw_content }
            // 更新邮件列表
            setMails(prevMails => prevMails.map(m =>
              m.id === mail.id ? mailData : m
            ))
          }
        } catch (error) {
          console.error('加载邮件详情失败:', error)
        }
      }

      const settings = getSettings()
      const folderName = formatFolderName(settings.folderNameFormat, mailData)
      const mailHash = await generateMailHash(mailData)

      const requestData = {
        mail_id: mailData.id,
        subject: mailData.subject,
        date: mailData.date,
        from_addr: mailData.from,
        body: mailData.body || '',
        base_path: settings.folderPath,
        folder_name: folderName,
        use_sub_folder: settings.useSubFolder,
        sub_folder_name: settings.subFolderName,
        save_mail_content: settings.saveMailContent,
        mail_content_file_name: settings.mailContentFileName,
        save_formats: settings.saveFormats || ['txt'],
        raw_content: mailData.raw_content || '',
        attachments: mailData.attachments || [],
        // 归属信息
        department: target?.type === 'department' ? target.name : null,
        project: target?.type === 'project' ? target.name : null,
        source: '邮件',
        hash: mailHash,
        sop_template_id: sopTemplateId || 'default-task'
      }

      // 始终使用 createWithAttachments，如果有附件会自动下载
      const result = await folderApi.createWithAttachments(requestData)
      message.success(result.message)

      // 更新已生成 hash 映射
      markGenerated(mailHash)
    } catch (error) {
      message.error(error.response?.data?.detail || '创建文件夹失败')
    } finally {
      setCreating(prev => ({ ...prev, [mail.id]: false }))
    }
  }

  // 预览邮件（先加载详情）
  const handlePreviewMail = async (mail) => {
    if (!mail.body) {
      setLoadingDetail(true)
      try {
        const result = await mailApi.getMailDetail(mail.id)
        if (result.success && result.data) {
          const updatedMail = { ...mail, body: result.data.body, attachments: result.data.attachments, raw_content: result.data.raw_content }
          // 更新邮件列表
          setMails(prevMails => prevMails.map(m =>
            m.id === mail.id ? updatedMail : m
          ))
          setPreviewMail(updatedMail)
        } else {
          setPreviewMail(mail)
        }
      } catch (error) {
        console.error('加载邮件详情失败:', error)
        message.error('加载邮件详情失败')
        setPreviewMail(mail)
      } finally {
        setLoadingDetail(false)
      }
    } else {
      setPreviewMail(mail)
    }
  }

  const handlePreviewCreate = (mail) => {
    setPreviewMail(null)
    openDeptModal(mail)
  }

  if (loading) {
    return (
      <div className="loading-container">
        <Spin size="large" />
        <div style={{ marginTop: 16, color: '#999' }}>加载邮件列表中...</div>
      </div>
    )
  }

  return (
    <div className="mail-list">
      <div className="mail-list-container">
        <div className="mail-list-content">
          {showRefreshTip && lastRefreshAt > 0 && (
            <div className="refresh-time-tip">
              <Tag color="processing">{`最近刷新：${formatRefreshTime(lastRefreshAt)}`}</Tag>
            </div>
          )}
          {/* 连接错误提示 */}
          {connectionError && (
            <Alert
              message={
                connectionError === 'network'
                  ? "无法连接到后端服务"
                  : connectionError === 'not_configured'
                    ? "尚未连接邮箱"
                    : connectionError === 'auth'
                      ? "邮件服务器认证失败"
                      : "获取邮件失败"
              }
              description={
                connectionError === 'network' ? (
                  <div>
                    <p>后端服务未能正常启动，请尝试以下操作：</p>
                    <ol style={{ paddingLeft: 20, margin: '8px 0' }}>
                      <li>重启应用程序</li>
                      <li>检查是否有杀毒软件阻止了后端进程</li>
                      <li>如问题持续，请查看应用日志或联系技术支持</li>
                    </ol>
                  </div>
                ) : connectionError === 'not_configured' ? (
                  <p>请点击左下角「设置」配置邮件服务器，连接成功后即可查看邮件列表。</p>
                ) : connectionError === 'auth' ? (
                  <div>
                    <p>邮件服务器连接失败，请检查您的配置：</p>
                    <ol style={{ paddingLeft: 20, margin: '8px 0' }}>
                      <li>点击左下角「设置」检查服务器地址和端口</li>
                      <li>确认用户名和密码正确</li>
                      <li>如使用企业邮箱，可能需要开启 IMAP 服务或使用授权码</li>
                    </ol>
                  </div>
                ) : (
                  <p>请稍后重试，或点击左下角「设置」检查邮件服务器配置。</p>
                )
              }
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
              action={
                <Button size="small" icon={<SettingOutlined />} onClick={() => window.dispatchEvent(new CustomEvent('openSettings'))}>
                  打开设置
                </Button>
              }
            />
          )}

          {mails.length === 0 && !connectionError ? (
            <Empty
              description={
                <div>
                  <p style={{ margin: '0 0 8px 0' }}>{`已连接成功，但最近 ${fetchDays} 天内暂无邮件`}</p>
                  <p style={{ margin: 0, fontSize: '12px', color: '#888' }}>（可点击左下角「设置」修改获取时间范围）</p>
                </div>
              }
            />
          ) : mails.length > 0 && (
            <List
              dataSource={mails}
              renderItem={(mail) => (
                <MailItemCard
                  key={mail.id}
                  mail={mail}
                  status={getGeneratedStatus(mail)}
                  creating={creating[mail.id]}
                  loadingDetail={loadingDetail}
                  onPreview={handlePreviewMail}
                  onCreate={openDeptModal}
                />
              )}
            />
          )}
        </div>

        {/* 右侧侧边栏：刷新按钮 + 时间滚动条 */}
        <div className="mail-sidebar">
          <Tooltip title="刷新列表" placement="left">
            <Button
              shape="circle"
              icon={<ReloadOutlined />}
              onClick={() => fetchMails({ onRefreshed: showRefreshHint })}
              loading={refreshing}
              className="refresh-btn"
            />
          </Tooltip>
          {timelineData.length > 0 && (
            <div className="mail-timeline">
              <div className="timeline-track"></div>
              {timelineData.map((item) => (
                <Tooltip title={item.label} placement="left" key={item.key}>
                  <div
                    className={`timeline-dot ${activeMonthKey === item.key ? 'active' : ''}`}
                    onClick={() => scrollToMail(item.mailId)}
                  >
                    <div className="timeline-dot-inner"></div>
                  </div>
                </Tooltip>
              ))}
            </div>
          )}
        </div>
      </div>

      <MailPreviewModal
        mail={previewMail}
        onClose={() => setPreviewMail(null)}
        onCreate={handlePreviewCreate}
      />

      {/* 部门选择弹窗 */}
      <DepartmentSelectModal
        open={deptModalOpen}
        mail={selectedMailForFolder}
        onConfirm={handleDeptConfirm}
        onCancel={() => {
          setDeptModalOpen(false)
          setSelectedMailForFolder(null)
        }}
        enableSop
      />
    </div>
  )
}

export default MailList
