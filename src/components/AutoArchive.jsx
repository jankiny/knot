import { useState, useEffect, useMemo } from 'react'
import { Button, Empty, Spin, message, Tag, Modal, Select, Tooltip, Space, Input } from 'antd'
import { SearchOutlined, SendOutlined, EditOutlined, EyeOutlined } from '@ant-design/icons'
import { archiveApi } from '../services/api'
import { getSettings, getDepartments, getDepartmentById } from '../services/settings'
import FolderCard from './FolderCard'
import './AutoArchive.css'

function AutoArchive() {
  const [loading, setLoading] = useState(false)
  const [folders, setFolders] = useState([])
  const [scanned, setScanned] = useState(false)
  const [departments, setDepartments] = useState([])
  const [activeMonthKey, setActiveMonthKey] = useState('')

  // 编辑部门弹窗
  const [editDeptVisible, setEditDeptVisible] = useState(false)
  const [editingFolder, setEditingFolder] = useState(null)
  const [editDeptId, setEditDeptId] = useState(null)

  // 查看/编辑内容弹窗
  const [contentVisible, setContentVisible] = useState(false)
  const [contentFolder, setContentFolder] = useState(null)
  const [editContent, setEditContent] = useState('')

  useEffect(() => {
    setDepartments(getDepartments())
    // 进入页面自动扫描一次（静默模式，不弹提示）
    handleScan(true)
  }, [])

  // 计算时间轴数据（按月份分组）
  const timelineData = useMemo(() => {
    if (!folders || folders.length === 0) return []

    const monthMap = new Map()
    folders.forEach((folder) => {
      const createTime = folder.create_time || ''
      if (!createTime) return
      // 从 "2026-02-25 10:00" 格式中提取年月
      const match = createTime.match(/^(\d{4})-(\d{2})/)
      if (!match) return
      const year = match[1]
      const month = match[2]
      const monthKey = `${year}-${month}`
      const monthLabel = `${year}年${parseInt(month)}月`

      if (!monthMap.has(monthKey)) {
        monthMap.set(monthKey, {
          key: monthKey,
          label: monthLabel,
          folderPath: folder.path,
          timestamp: new Date(createTime).getTime() || 0
        })
      }
    })

    return Array.from(monthMap.values()).sort((a, b) => b.timestamp - a.timestamp)
  }, [folders])

  // 监听滚动高亮月份
  useEffect(() => {
    if (!folders || folders.length === 0 || timelineData.length === 0) return

    let visibleFolders = new Map()

    const observer = new IntersectionObserver((entries) => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          visibleFolders.set(entry.target.id, entry.boundingClientRect.top)
        } else {
          visibleFolders.delete(entry.target.id)
        }
      })

      if (visibleFolders.size > 0) {
        let closestId = ''
        let minTop = Infinity

        for (const [id, top] of visibleFolders.entries()) {
          const adjustedTop = top - 80
          if (adjustedTop >= 0 && adjustedTop < minTop) {
            minTop = adjustedTop
            closestId = id
          }
        }
        if (!closestId) {
          let maxTop = -Infinity
          for (const [id, top] of visibleFolders.entries()) {
            if (top > maxTop) {
              maxTop = top
              closestId = id
            }
          }
        }

        if (closestId) {
          const folderPath = closestId.replace('folder-', '')
          const folder = folders.find(f => f.path === folderPath)
          if (folder && folder.create_time) {
            const match = folder.create_time.match(/^(\d{4})-(\d{2})/)
            if (match) {
              setActiveMonthKey(`${match[1]}-${match[2]}`)
            }
          }
        }
      }
    }, {
      rootMargin: '-80px 0px 0px 0px',
      threshold: [0, 0.1, 0.5, 0.9, 1]
    })

    folders.forEach(folder => {
      const el = document.getElementById(`folder-${folder.path}`)
      if (el) observer.observe(el)
    })

    return () => observer.disconnect()
  }, [folders, timelineData])

  const scrollToFolder = (folderPath) => {
    const element = document.getElementById(`folder-${folderPath}`)
    if (element) {
      element.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }

  const handleScan = async (silent = false) => {
    const settings = getSettings()
    const scanPath = settings.folderPath || '~/Desktop'

    setLoading(true)
    setFolders([])

    try {
      const result = await archiveApi.scan(scanPath)
      if (result.success) {
        setFolders(result.folders || [])
        setScanned(true)
        if (!silent) {
          if (result.folders.length === 0) {
            message.info('未找到含工作记录的文件夹')
          } else {
            message.success(`找到 ${result.folders.length} 个工作文件夹`)
          }
        }
      }
    } catch (error) {
      if (!silent) {
        message.error(error.response?.data?.detail || '扫描失败')
      }
      setScanned(true)
    } finally {
      setLoading(false)
    }
  }

  // 归档操作
  const handleArchive = async (folder) => {
    const dept = departments.find(d => d.name === folder.department)
    if (!dept) {
      message.warning('请先编辑归属部门后再归档')
      return
    }

    Modal.confirm({
      title: '确认归档',
      content: (
        <div>
          <p>将文件夹移动到归档目录：</p>
          <p><strong>{folder.name}</strong></p>
          <p>→ {dept.archivePath}/{folder.name.substring(0, 4)}/</p>
        </div>
      ),
      okText: '确认归档',
      cancelText: '取消',
      onOk: async () => {
        try {
          const result = await archiveApi.move(folder.path, dept.archivePath)
          if (result.success) {
            message.success(result.message || '归档成功')
            setFolders(prev => prev.filter(f => f.path !== folder.path))
          }
        } catch (error) {
          message.error(error.response?.data?.detail || '归档失败')
        }
      }
    })
  }

  // 编辑部门
  const handleEditDept = (folder) => {
    setEditingFolder(folder)
    const dept = departments.find(d => d.name === folder.department)
    setEditDeptId(dept ? dept.id : null)
    setEditDeptVisible(true)
  }

  const handleDeptSave = async () => {
    if (!editDeptId || !editingFolder) return
    const dept = getDepartmentById(editDeptId)
    if (!dept) return

    try {
      await archiveApi.updateWorkRecord(editingFolder.path, dept.name, '')
      message.success('归属部门已更新')
      setFolders(prev => prev.map(f =>
        f.path === editingFolder.path ? { ...f, department: dept.name } : f
      ))
      setEditDeptVisible(false)
      setEditingFolder(null)
    } catch (error) {
      message.error(error.response?.data?.detail || '更新失败')
    }
  }

  // 查看/编辑内容
  const handleViewContent = (folder) => {
    setContentFolder(folder)
    setEditContent(folder.content || '')
    setContentVisible(true)
  }

  const handleContentSave = async () => {
    if (!contentFolder) return

    try {
      await archiveApi.updateWorkRecord(contentFolder.path, '', editContent)
      message.success('工作记录已更新')
      setFolders(prev => prev.map(f =>
        f.path === contentFolder.path ? { ...f, content: editContent } : f
      ))
      setContentVisible(false)
      setContentFolder(null)
    } catch (error) {
      message.error(error.response?.data?.detail || '更新失败')
    }
  }

  const settings = getSettings()

  if (loading) {
    return (
      <div className="loading-container">
        <Spin size="large" />
        <div style={{ marginTop: 16, color: '#999' }}>扫描工作文件夹中...</div>
      </div>
    )
  }

  return (
    <div className="auto-archive">
      <div className="archive-list-container">
        <div className="archive-list-content">
          {!scanned && folders.length === 0 && (
            <Empty
              description={
                <div>
                  <p style={{ margin: '0 0 8px 0' }}>当前工作目录：{settings.folderPath || '~/Desktop'}</p>
                  <p style={{ margin: 0, fontSize: '12px', color: '#888' }}>
                    点击右侧扫描按钮查找工作文件夹，或在「设置 → 文件夹设置 → 工作目录」中修改扫描位置
                  </p>
                </div>
              }
            />
          )}

          {scanned && folders.length === 0 && (
            <Empty
              description={
                <div>
                  <p style={{ margin: '0 0 8px 0' }}>工作目录中暂无含工作记录的文件夹</p>
                  <p style={{ margin: 0, fontSize: '12px', color: '#888' }}>
                    请先在邮件列表或快速创建中生成工作文件夹后再尝试扫描
                  </p>
                </div>
              }
            />
          )}

          {folders.length > 0 && (
            <div className="folders-list">
              {folders.map(folder => (
                <div key={folder.path} id={`folder-${folder.path}`} style={{ scrollMarginTop: 80 }}>
                  <FolderCard
                    folder={folder}
                    departments={departments}
                    onArchive={handleArchive}
                    onEditDept={handleEditDept}
                    onViewContent={handleViewContent}
                  />
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 右侧侧边栏：扫描按钮 + 时间滚动条 */}
        <div className="archive-sidebar">
          <Tooltip title="扫描工作文件夹" placement="left">
            <Button
              shape="circle"
              icon={<SearchOutlined />}
              onClick={handleScan}
              className="scan-btn"
            />
          </Tooltip>
          {timelineData.length > 0 && (
            <div className="archive-timeline">
              <div className="timeline-track"></div>
              {timelineData.map((item) => (
                <Tooltip title={item.label} placement="left" key={item.key}>
                  <div
                    className={`timeline-dot ${activeMonthKey === item.key ? 'active' : ''}`}
                    onClick={() => scrollToFolder(item.folderPath)}
                  >
                    <div className="timeline-dot-inner"></div>
                  </div>
                </Tooltip>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* 编辑部门弹窗 */}
      <Modal
        title="编辑归属部门"
        open={editDeptVisible}
        onOk={handleDeptSave}
        onCancel={() => { setEditDeptVisible(false); setEditingFolder(null) }}
        okText="保存"
        cancelText="取消"
        width={400}
      >
        {editingFolder && (
          <div>
            <p style={{ marginBottom: 12 }}>文件夹：<strong>{editingFolder.name}</strong></p>
            <Select
              style={{ width: '100%' }}
              placeholder="请选择部门"
              value={editDeptId}
              onChange={setEditDeptId}
              options={departments.map(d => ({
                label: d.name,
                value: d.id,
                desc: d.archivePath
              }))}
              optionRender={(option) => (
                <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span>{option.data.label}</span>
                  <span style={{ color: '#999', fontSize: 12 }}>{option.data.desc}</span>
                </div>
              )}
            />
          </div>
        )}
      </Modal>

      {/* 查看/编辑内容弹窗 */}
      <Modal
        title={contentFolder ? `工作记录 - ${contentFolder.name}` : '工作记录'}
        open={contentVisible}
        onOk={handleContentSave}
        onCancel={() => { setContentVisible(false); setContentFolder(null) }}
        okText="保存"
        cancelText="取消"
        width={600}
      >
        {contentFolder && (
          <div>
            <div style={{ marginBottom: 12, color: '#666', fontSize: 13 }}>
              <Space>
                {contentFolder.source && <Tag>{contentFolder.source}</Tag>}
                {contentFolder.create_time && <span>创建：{contentFolder.create_time}</span>}
              </Space>
            </div>
            <Input.TextArea
              value={editContent}
              onChange={(e) => setEditContent(e.target.value)}
              rows={10}
              placeholder="请记录工作过程..."
              style={{ fontFamily: 'monospace' }}
            />
          </div>
        )}
      </Modal>
    </div>
  )
}

export default AutoArchive
