import { useState, useEffect, useMemo, useRef } from 'react'
import { List, Card, Button, Tag, Collapse, message, Spin, Empty, Tooltip, Modal, Alert } from 'antd'
import { FolderAddOutlined, PaperClipOutlined, ReloadOutlined, EyeOutlined, SettingOutlined, CheckCircleOutlined, InboxOutlined } from '@ant-design/icons'
import { mailApi, folderApi, archiveApi, USE_MOCK } from '../services/api'
import { getSettings, formatFolderName, cleanSubjectForFolder, generateMailHash, getDepartments } from '../services/settings'
import DepartmentSelectModal from './DepartmentSelectModal'
import './MailList.css'

function MailList() {
  const [mails, setMails] = useState([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState({})
  const [previewMail, setPreviewMail] = useState(null)
  const [loadingDetail, setLoadingDetail] = useState(false)
  // 当前处于视口视野内的主要月份
  const [activeMonthKey, setActiveMonthKey] = useState('')
  // 部门选择弹窗状态
  const [deptModalOpen, setDeptModalOpen] = useState(false)
  const [selectedMailForFolder, setSelectedMailForFolder] = useState(null)
  // 连接状态
  const [connectionError, setConnectionError] = useState(null)
  // 已生成的邮件 hash 映射：{ mailHash: 'working' | 'archived' }
  const [generatedHashMap, setGeneratedHashMap] = useState({})
  // 邮件 id -> hash 缓存（用于同步渲染中查找状态）
  const [mailHashCache, setMailHashCache] = useState({})

  // 记录最近一次获取数据的天数范围
  const [fetchDays, setFetchDays] = useState(7)

  // 用于防止 StrictMode 双重调用导致的竞态条件
  const fetchIdRef = useRef(0)

  const fetchMails = async () => {
    // 递增 fetchId，用于忽略过时的请求结果（防止 StrictMode 双重调用竞态）
    const currentFetchId = ++fetchIdRef.current
    setLoading(true)
    setConnectionError(null)
    try {
      const settings = getSettings()

      // 自动连接：如果有保存的邮箱配置，先尝试连接后端
      if (settings.mailServer && settings.mailUsername && settings.mailPasswordEncrypted) {
        try {
          let password = ''
          if (window.electronAPI?.decryptPassword) {
            password = await window.electronAPI.decryptPassword(settings.mailPasswordEncrypted) || ''
          }
          if (password) {
            await mailApi.connect({
              server: settings.mailServer,
              port: settings.mailPort || 993,
              username: settings.mailUsername,
              password: password,
              use_ssl: settings.mailUseSsl !== false
            })
          }
        } catch (connectError) {
          console.error('自动连接邮箱失败:', connectError)
          // 连接失败不阻断，让后续 getMailList 决定错误类型
        }
      }

      // 如果在等待期间又触发了新的 fetchMails，放弃本次结果
      if (currentFetchId !== fetchIdRef.current) return

      // 使用设置中的 limit 和 days，如果未设置则使用默认值
      const limit = settings.mailLimit || 50
      const days = settings.mailDays !== undefined ? settings.mailDays : 7

      setFetchDays(days)

      const result = await mailApi.getMailList(limit, days)
      // 再次检查：如果在请求期间又触发了新的 fetchMails，忽略旧结果
      if (currentFetchId !== fetchIdRef.current) return
      const mailData = result.data || []
      setMails(mailData)

      // 加载已生成状态：通过后端扫描工作目录获取已有的 hash
      loadGeneratedHashes(mailData, settings)
    } catch (error) {
      // 如果是过时的请求，忽略其错误
      if (currentFetchId !== fetchIdRef.current) return
      if (error.code === 'ERR_NETWORK') {
        setConnectionError('network')
      } else if (error.response?.status === 400) {
        // 400 表示后端正常但未配置邮箱连接
        setConnectionError('not_configured')
      } else if (error.response?.status === 401 || error.response?.status === 403) {
        setConnectionError('auth')
      } else {
        setConnectionError('unknown')
      }
      setMails([])
    } finally {
      if (currentFetchId === fetchIdRef.current) {
        setLoading(false)
      }
    }
  }

  useEffect(() => {
    fetchMails()
  }, [])

  // 加载已生成状态：扫描工作目录和归档目录，构建 hash -> status 映射
  const loadGeneratedHashes = async (mailList, settings) => {
    try {
      // 为所有邮件预计算 hash
      const hashCacheEntries = await Promise.all(
        mailList.map(async (mail) => {
          const hash = await generateMailHash(mail)
          return [mail.id, hash]
        })
      )
      const newMailHashCache = Object.fromEntries(hashCacheEntries)
      setMailHashCache(newMailHashCache)

      // 扫描工作目录
      const scanResult = await archiveApi.scan(settings.scanPath || settings.folderPath)
      const hashMap = {}
      if (scanResult.success && scanResult.folders) {
        scanResult.folders.forEach(f => {
          if (f.hash) hashMap[f.hash] = 'working'
        })
      }

      // 扫描各部门归档目录
      const departments = getDepartments()
      for (const dept of departments) {
        if (dept.archivePath) {
          try {
            const archiveResult = await archiveApi.scan(dept.archivePath, true)
            if (archiveResult.success && archiveResult.folders) {
              archiveResult.folders.forEach(f => {
                if (f.hash) hashMap[f.hash] = 'archived'
              })
            }
          } catch {
            // 归档目录不存在时忽略
          }
        }
      }

      setGeneratedHashMap(hashMap)
    } catch (err) {
      console.error('加载已生成状态失败:', err)
    }
  }

  // 计算时间轴数据
  const timelineData = useMemo(() => {
    if (!mails || mails.length === 0) return []

    const monthMap = new Map()

    // mails 通常是按时间倒序排列的（最新的在前）
    mails.forEach((mail) => {
      if (!mail.date) return
      const date = new Date(mail.date)
      if (isNaN(date.getTime())) return

      const year = date.getFullYear()
      const month = date.getMonth() + 1
      const monthKey = `${year}-${month.toString().padStart(2, '0')}`
      const monthLabel = `${year}年${month}月`

      // 因为邮件是按时间倒序的，所以我们遍历时遇到的第一个该月份的邮件就是该月份最新的邮件
      if (!monthMap.has(monthKey)) {
        monthMap.set(monthKey, {
          key: monthKey,
          label: monthLabel,
          mailId: mail.id,
          timestamp: date.getTime()
        })
      }
    })

    // 按时间倒序排列月份节点（最新的月份在上）
    return Array.from(monthMap.values()).sort((a, b) => b.timestamp - a.timestamp)
  }, [mails])

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
            const date = new Date(mail.date)
            const year = date.getFullYear()
            const month = date.getMonth() + 1
            const monthKey = `${year}-${month.toString().padStart(2, '0')}`
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
      const archivePaths = departments.map(d => d.archivePath).filter(Boolean)

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

  // 确认选择部门后创建文件夹
  const handleDeptConfirm = async (department) => {
    setDeptModalOpen(false)
    if (selectedMailForFolder) {
      await handleCreateFolder(selectedMailForFolder, department)
    }
    setSelectedMailForFolder(null)
  }

  // 创建文件夹（始终包含附件下载）
  const handleCreateFolder = async (mail, department = null) => {
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
        // 部门信息
        department: department ? department.name : null,
        source: '邮件',
        hash: mailHash
      }

      // 始终使用 createWithAttachments，如果有附件会自动下载
      const result = await folderApi.createWithAttachments(requestData)
      message.success(result.message)

      // 更新已生成 hash 映射
      setGeneratedHashMap(prev => ({ ...prev, [mailHash]: 'working' }))
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

  const formatDate = (dateStr) => {
    if (!dateStr) return ''
    try {
      const date = new Date(dateStr)
      return date.toLocaleDateString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit'
      })
    } catch {
      return dateStr
    }
  }

  const formatFileSize = (bytes) => {
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  }

  // 截取邮件正文预览
  const getBodyPreview = (body, maxLength = 100) => {
    if (!body) return ''
    const text = body.replace(/\n+/g, ' ').trim()
    if (text.length <= maxLength) return text
    return text.slice(0, maxLength) + '...'
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
                <Card className="mail-item" key={mail.id} id={`mail-${mail.id}`}>
                  <div className="mail-content">
                    <div className="mail-info">
                      <div className="mail-subject">{mail.subject}</div>
                      <div className="mail-meta">
                        <span className="mail-from">{mail.from}</span>
                        <span className="mail-date">{formatDate(mail.date)}</span>
                      </div>
                      {mail.body && (
                        <div className="mail-body-preview">
                          {getBodyPreview(mail.body)}
                        </div>
                      )}
                    </div>

                    <div className="mail-actions">
                      {(() => {
                        const mailHash = mailHashCache[mail.id]
                        const status = mailHash && generatedHashMap[mailHash]
                        if (status === 'archived') {
                          return (
                            <Tag icon={<InboxOutlined />} color="default">
                              已归档
                            </Tag>
                          )
                        } else if (status === 'working') {
                          return (
                            <Tag icon={<CheckCircleOutlined />} color="success">
                              已生成
                            </Tag>
                          )
                        }
                        return null
                      })()}

                      {(mail.attachment_count > 0 || mail.has_attachments) && (
                        <Tag icon={<PaperClipOutlined />} color="blue">
                          {mail.attachment_count > 0 ? `${mail.attachment_count} 个附件` : '有附件'}
                        </Tag>
                      )}

                      <Tooltip title="预览邮件">
                        <Button
                          icon={<EyeOutlined />}
                          onClick={() => handlePreviewMail(mail)}
                          loading={loadingDetail}
                        />
                      </Tooltip>

                      <Tooltip title={(mail.attachment_count > 0 || mail.has_attachments) ? "创建文件夹并下载附件" : "创建文件夹"}>
                        <Button
                          type="primary"
                          icon={<FolderAddOutlined />}
                          onClick={() => openDeptModal(mail)}
                          loading={creating[mail.id]}
                        >
                          生成
                        </Button>
                      </Tooltip>
                    </div>
                  </div>

                  {mail.attachments && mail.attachments.length > 0 && (
                    <Collapse
                      ghost
                      className="attachments-collapse"
                      items={[{
                        key: '1',
                        label: '查看附件详情',
                        children: (
                          <ul className="attachment-list">
                            {mail.attachments.map((att, idx) => (
                              <li key={idx}>
                                <PaperClipOutlined />
                                <span className="att-name">{att.filename}</span>
                                <span className="att-size">{formatFileSize(att.size)}</span>
                              </li>
                            ))}
                          </ul>
                        )
                      }]}
                    />
                  )}
                </Card>
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
              onClick={fetchMails}
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

      {/* 邮件预览弹窗 */}
      <Modal
        title={previewMail?.subject}
        open={!!previewMail}
        onCancel={() => setPreviewMail(null)}
        footer={[
          <Button key="close" onClick={() => setPreviewMail(null)}>
            关闭
          </Button>,
          <Button
            key="create"
            type="primary"
            icon={<FolderAddOutlined />}
            onClick={() => {
              setPreviewMail(null)
              openDeptModal(previewMail)
            }}
          >
            生成文件夹
          </Button>
        ]}
        width={700}
      >
        {previewMail && (
          <div className="mail-preview">
            <div className="preview-header">
              <div className="preview-meta">
                <span className="label">发件人：</span>
                <span className="value">{previewMail.from}</span>
              </div>
              <div className="preview-meta">
                <span className="label">日期：</span>
                <span className="value">{formatDate(previewMail.date)}</span>
              </div>
              {previewMail.attachment_count > 0 && (
                <div className="preview-meta">
                  <span className="label">附件：</span>
                  <span className="value">{previewMail.attachment_count} 个</span>
                </div>
              )}
            </div>
            <div className="preview-body">
              {previewMail.body || '(无正文内容)'}
            </div>
            {previewMail.attachments && previewMail.attachments.length > 0 && (
              <div className="preview-attachments">
                <div className="attachments-title">附件列表：</div>
                <ul>
                  {previewMail.attachments.map((att, idx) => (
                    <li key={idx}>
                      <PaperClipOutlined /> {att.filename} ({formatFileSize(att.size)})
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </Modal>

      {/* 部门选择弹窗 */}
      <DepartmentSelectModal
        open={deptModalOpen}
        mail={selectedMailForFolder}
        onConfirm={handleDeptConfirm}
        onCancel={() => {
          setDeptModalOpen(false)
          setSelectedMailForFolder(null)
        }}
      />
    </div>
  )
}

export default MailList
