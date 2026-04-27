import { useState } from 'react'
import { Button, Empty, Input, List, Popconfirm, Space, Tag, message } from 'antd'
import {
  CheckOutlined,
  CloseOutlined,
  DeleteOutlined,
  EditOutlined,
  FolderOutlined,
  PlusOutlined,
  StarFilled,
  StarOutlined
} from '@ant-design/icons'
import {
  addProject,
  deleteProject,
  getProjects,
  getSettings,
  setDefaultProject,
  updateProject
} from '../services/settings'
import './DepartmentManager.css'

function ProjectManager({ onUpdate }) {
  const [projects, setProjects] = useState(getProjects())
  const [adding, setAdding] = useState(false)
  const [editingId, setEditingId] = useState(null)
  const [newName, setNewName] = useState('')
  const [newPath, setNewPath] = useState('')
  const [editName, setEditName] = useState('')
  const [editPath, setEditPath] = useState('')

  const settings = getSettings()
  const defaultProjectId = settings.defaultProjectId

  const refreshProjects = () => {
    setProjects(getProjects())
    onUpdate?.()
  }

  const handleAdd = () => {
    if (!newName.trim()) {
      message.warning('请输入项目名称')
      return
    }
    if (!newPath.trim()) {
      message.warning('请输入归档路径')
      return
    }
    addProject(newName.trim(), newPath.trim(), false)
    setNewName('')
    setNewPath('')
    setAdding(false)
    refreshProjects()
    message.success('项目添加成功')
  }

  const handleEdit = (project) => {
    setEditingId(project.id)
    setEditName(project.name)
    setEditPath(project.archivePath)
  }

  const handleSaveEdit = () => {
    if (!editName.trim()) {
      message.warning('项目名称不能为空')
      return
    }
    if (!editPath.trim()) {
      message.warning('归档路径不能为空')
      return
    }
    updateProject(editingId, {
      name: editName.trim(),
      archivePath: editPath.trim(),
      useYearFolder: false
    })
    setEditingId(null)
    refreshProjects()
    message.success('项目更新成功')
  }

  const handleDelete = (id) => {
    deleteProject(id)
    refreshProjects()
    message.success('项目已删除')
  }

  const handleSetDefault = (id) => {
    setDefaultProject(id)
    refreshProjects()
    message.success('已设为默认项目')
  }

  const handleSelectFolder = async (callback) => {
    if (window.electronAPI?.selectFolder) {
      const selectedPath = await window.electronAPI.selectFolder()
      if (selectedPath) callback(selectedPath)
    } else {
      message.info('请手动输入路径，或在 Electron 应用中使用文件夹选择')
    }
  }

  return (
    <div className="department-manager">
      <div className="dept-header">
        <span className="dept-title">项目列表</span>
        {!adding && (
          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => setAdding(true)}>
            添加项目
          </Button>
        )}
      </div>

      {adding && (
        <div className="dept-add-form">
          <Input
            placeholder="项目名称，如 大培训"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            style={{ marginBottom: 8 }}
          />
          <Space.Compact style={{ width: '100%', marginBottom: 8 }}>
            <Input
              prefix={<FolderOutlined />}
              placeholder="归档路径，如 D:/归档/大培训"
              value={newPath}
              onChange={(e) => setNewPath(e.target.value)}
            />
            <Button onClick={() => handleSelectFolder(setNewPath)}>选择</Button>
          </Space.Compact>
          <Space>
            <Button type="primary" size="small" icon={<CheckOutlined />} onClick={handleAdd}>
              确定
            </Button>
            <Button size="small" icon={<CloseOutlined />} onClick={() => setAdding(false)}>
              取消
            </Button>
          </Space>
        </div>
      )}

      {projects.length === 0 && !adding ? (
        <Empty description="暂无项目，请添加" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <List
          className="dept-list"
          dataSource={projects}
          renderItem={(project) => (
            <List.Item className="dept-item">
              {editingId === project.id ? (
                <div className="dept-edit-form">
                  <Input
                    placeholder="项目名称"
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                    style={{ marginBottom: 8 }}
                  />
                  <Space.Compact style={{ width: '100%', marginBottom: 8 }}>
                    <Input
                      prefix={<FolderOutlined />}
                      placeholder="归档路径"
                      value={editPath}
                      onChange={(e) => setEditPath(e.target.value)}
                    />
                    <Button onClick={() => handleSelectFolder(setEditPath)}>选择</Button>
                  </Space.Compact>
                  <Space>
                    <Button type="primary" size="small" icon={<CheckOutlined />} onClick={handleSaveEdit}>
                      保存
                    </Button>
                    <Button size="small" icon={<CloseOutlined />} onClick={() => setEditingId(null)}>
                      取消
                    </Button>
                  </Space>
                </div>
              ) : (
                <div className="dept-info">
                  <div className="dept-main">
                    <span className="dept-name">
                      {project.name}
                      {defaultProjectId === project.id && <Tag color="gold" style={{ marginLeft: 8 }}>默认</Tag>}
                      <Tag color="default" style={{ marginLeft: 8 }}>直接归档</Tag>
                    </span>
                    <span className="dept-path">{project.archivePath}</span>
                  </div>
                  <Space className="dept-actions">
                    <Button
                      type="text"
                      size="small"
                      icon={defaultProjectId === project.id ? <StarFilled style={{ color: '#faad14' }} /> : <StarOutlined />}
                      onClick={() => handleSetDefault(project.id)}
                      title="设为默认"
                    />
                    <Button type="text" size="small" icon={<EditOutlined />} onClick={() => handleEdit(project)} />
                    <Popconfirm
                      title="确定删除此项目？"
                      onConfirm={() => handleDelete(project.id)}
                      okText="删除"
                      cancelText="取消"
                    >
                      <Button type="text" size="small" danger icon={<DeleteOutlined />} />
                    </Popconfirm>
                  </Space>
                </div>
              )}
            </List.Item>
          )}
        />
      )}
    </div>
  )
}

export default ProjectManager
