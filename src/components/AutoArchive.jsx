import { useState, useEffect } from 'react'
import { Button, Space, Empty, Spin, message, Alert, Input, Checkbox } from 'antd'
import { ScanOutlined, FolderOpenOutlined, SendOutlined, FolderOutlined } from '@ant-design/icons'
import { archiveApi } from '../services/api'
import { getSettings, saveSettings, getDepartments, getDepartmentById } from '../services/settings'
import FolderCard from './FolderCard'
import './AutoArchive.css'

function AutoArchive() {
  const [loading, setLoading] = useState(false)
  const [archiving, setArchiving] = useState(false)
  const [folders, setFolders] = useState([])
  const [scanPath, setScanPath] = useState('')
  const [selectedFolders, setSelectedFolders] = useState({}) // { path: true }
  const [folderDepts, setFolderDepts] = useState({}) // { path: deptId }
  const [departments, setDepartments] = useState([])

  useEffect(() => {
    const settings = getSettings()
    setScanPath(settings.scanPath || '~/Desktop')
    setDepartments(getDepartments())
  }, [])

  const handleScan = async () => {
    if (!scanPath.trim()) {
      message.warning('请输入扫描目录')
      return
    }

    setLoading(true)
    setFolders([])
    setSelectedFolders({})
    setFolderDepts({})

    try {
      const result = await archiveApi.scan(scanPath)
      if (result.success) {
        setFolders(result.folders || [])
        // 自动根据工作记录中的部门匹配
        const autoMatched = {}
        result.folders.forEach(folder => {
          if (folder.department) {
            const matchedDept = departments.find(d => d.name === folder.department)
            if (matchedDept) {
              autoMatched[folder.path] = matchedDept.id
            }
          }
        })
        setFolderDepts(autoMatched)

        if (result.folders.length === 0) {
          message.info('未找到符合格式的工作文件夹')
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

  const handleSelectFolder = (path, checked) => {
    setSelectedFolders(prev => ({
      ...prev,
      [path]: checked
    }))
  }

  const handleSelectAll = (checked) => {
    if (checked) {
      const all = {}
      folders.forEach(f => { all[f.path] = true })
      setSelectedFolders(all)
    } else {
      setSelectedFolders({})
    }
  }

  const handleDeptChange = (path, deptId) => {
    setFolderDepts(prev => ({
      ...prev,
      [path]: deptId
    }))
  }

  const handleArchive = async () => {
    const selectedPaths = Object.keys(selectedFolders).filter(p => selectedFolders[p])
    if (selectedPaths.length === 0) {
      message.warning('请选择要归档的文件夹')
      return
    }

    // 检查是否所有选中的都分配了部门
    const items = []
    for (const path of selectedPaths) {
      const deptId = folderDepts[path]
      if (!deptId) {
        message.warning('请为所有选中的文件夹分配部门')
        return
      }
      const dept = getDepartmentById(deptId)
      if (!dept) {
        message.warning('部门配置无效')
        return
      }
      items.push({
        folder_path: path,
        archive_path: dept.archivePath
      })
    }

    setArchiving(true)
    try {
      const result = await archiveApi.batchMove(items)
      if (result.success) {
        message.success(`归档完成：成功 ${result.success_count} 个，失败 ${result.fail_count} 个`)
        // 刷新列表
        handleScan()
      }
    } catch (error) {
      message.error(error.response?.data?.detail || '归档失败')
    } finally {
      setArchiving(false)
    }
  }

  const handleSelectScanFolder = async () => {
    if (window.electronAPI?.selectFolder) {
      const selectedPath = await window.electronAPI.selectFolder()
      if (selectedPath) {
        setScanPath(selectedPath)
        saveSettings({ scanPath: selectedPath })
      }
    } else {
      message.info('请手动输入路径，或在 Electron 应用中使用文件夹选择')
    }
  }

  const selectedCount = Object.values(selectedFolders).filter(Boolean).length
  const allSelected = folders.length > 0 && selectedCount === folders.length

  return (
    <div className="auto-archive">
      <div className="archive-header">
        <h2>自动归档</h2>
        <p className="header-desc">扫描桌面工作文件夹，选择后一键归档到对应部门目录</p>
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
        <>
          <div className="folders-toolbar">
            <Checkbox
              checked={allSelected}
              indeterminate={selectedCount > 0 && !allSelected}
              onChange={(e) => handleSelectAll(e.target.checked)}
            >
              全选 ({selectedCount}/{folders.length})
            </Checkbox>
            <Button
              type="primary"
              icon={<SendOutlined />}
              onClick={handleArchive}
              loading={archiving}
              disabled={selectedCount === 0 || departments.length === 0}
            >
              归档选中 ({selectedCount})
            </Button>
          </div>

          <div className="folders-list">
            {folders.map(folder => (
              <FolderCard
                key={folder.path}
                folder={folder}
                departments={departments}
                selected={!!selectedFolders[folder.path]}
                onSelect={handleSelectFolder}
                selectedDeptId={folderDepts[folder.path]}
                onDeptChange={handleDeptChange}
              />
            ))}
          </div>
        </>
      )}
    </div>
  )
}

export default AutoArchive
