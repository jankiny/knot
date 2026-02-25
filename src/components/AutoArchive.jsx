import { useState, useEffect } from 'react'
import { Button, Space, Empty, Spin, message, Alert, Input, Tag, Modal, Select, Tooltip } from 'antd'
import { ScanOutlined, FolderOpenOutlined, SendOutlined, FolderOutlined, EditOutlined, FileTextOutlined, QuestionCircleOutlined } from '@ant-design/icons'
import { archiveApi } from '../services/api'
import { getSettings, saveSettings, getDepartments, getDepartmentById } from '../services/settings'
import FolderCard from './FolderCard'
import './AutoArchive.css'

function AutoArchive() {
  const [loading, setLoading] = useState(false)
  const [folders, setFolders] = useState([])
  const [scanPath, setScanPath] = useState('')
  const [departments, setDepartments] = useState([])

  // 编辑部门弹窗
  const [editDeptVisible, setEditDeptVisible] = useState(false)
  const [editingFolder, setEditingFolder] = useState(null)
  const [editDeptId, setEditDeptId] = useState(null)

  // 查看/编辑内容弹窗
  const [contentVisible, setContentVisible] = useState(false)
  const [contentFolder, setContentFolder] = useState(null)
  const [editContent, setEditContent] = useState('')

  useEffect(() => {
    const settings = getSettings()
    setScanPath(settings.folderPath || '~/Desktop')
    setDepartments(getDepartments())
  }, [])

  const handleScan = async () => {
    if (!scanPath.trim()) {
      message.warning('请输入扫描目录')
      return
    }

    setLoading(true)
    setFolders([])

    try {
      const result = await archiveApi.scan(scanPath)
      if (result.success) {
        setFolders(result.folders || [])
        if (result.folders.length === 0) {
          message.info('未找到含工作记录的文件夹')
        } else {
          message.success(`找到 ${result.folders.length} 个工作文件夹`)
        }
      }
    } catch (error) {
      message.error(error.response?.data?.detail || '扫描失败')
    } finally {
      setLoading(false)
    }
  }

  const handleSelectScanFolder = async () => {
    if (window.electronAPI?.selectFolder) {
      const selectedPath = await window.electronAPI.selectFolder()
      if (selectedPath) {
        setScanPath(selectedPath)
        saveSettings({ folderPath: selectedPath })
      }
    } else {
      message.info('请手动输入路径，或在 Electron 应用中使用文件夹选择')
    }
  }

  // 归档操作
  const handleArchive = async (folder) => {
    // 需要部门信息来确定归档路径
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
            // 从列表中移除已归档的
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
      // 更新本地状态
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
      // 更新本地状态
      setFolders(prev => prev.map(f =>
        f.path === contentFolder.path ? { ...f, content: editContent } : f
      ))
      setContentVisible(false)
      setContentFolder(null)
    } catch (error) {
      message.error(error.response?.data?.detail || '更新失败')
    }
  }

  return (
    <div className="auto-archive">
      <div className="archive-header">
        <h2>自动归档</h2>
        <p className="header-desc">扫描工作目录中含工作记录的文件夹，管理和归档到对应部门目录</p>
      </div>

      <div className="scan-section">
        <Space.Compact style={{ width: '100%', maxWidth: 500 }}>
          <Input
            prefix={<FolderOutlined />}
            placeholder="扫描目录，如 ~/Desktop"
            value={scanPath}
            onChange={(e) => setScanPath(e.target.value)}
          />
          <Button onClick={handleSelectScanFolder}>选择</Button>
          <Button
            type="primary"
            icon={<ScanOutlined />}
            onClick={handleScan}
            loading={loading}
          >
            扫描
          </Button>
        </Space.Compact>
      </div>

      {departments.length === 0 && (
        <Alert
          type="warning"
          message="请先在设置中添加部门"
          description="归档功能需要配置部门及其归档路径"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {loading && (
        <div className="loading-container" style={{ textAlign: 'center', padding: '50px' }}>
          <Spin />
          <div style={{ marginTop: 16, color: '#999' }}>扫描中...</div>
        </div>
      )}

      {!loading && folders.length === 0 && (
        <Empty
          description="暂无数据，点击扫描按钮开始"
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        />
      )}

      {!loading && folders.length > 0 && (
        <div className="folders-list">
          {folders.map(folder => (
            <FolderCard
              key={folder.path}
              folder={folder}
              departments={departments}
              onArchive={handleArchive}
              onEditDept={handleEditDept}
              onViewContent={handleViewContent}
            />
          ))}
        </div>
      )}

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
