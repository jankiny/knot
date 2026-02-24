import { useState } from 'react'
import { List, Button, Input, Space, message, Popconfirm, Empty, Tag } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, FolderOutlined, CheckOutlined, CloseOutlined, StarOutlined, StarFilled } from '@ant-design/icons'
import { getDepartments, addDepartment, updateDepartment, deleteDepartment, getSettings, setDefaultDepartment } from '../services/settings'
import './DepartmentManager.css'

function DepartmentManager({ onUpdate }) {
  const [departments, setDepartments] = useState(getDepartments())
  const [adding, setAdding] = useState(false)
  const [editingId, setEditingId] = useState(null)
  const [newName, setNewName] = useState('')
  const [newPath, setNewPath] = useState('')
  const [editName, setEditName] = useState('')
  const [editPath, setEditPath] = useState('')

  const settings = getSettings()
  const defaultDeptId = settings.defaultDepartmentId

  const refreshDepartments = () => {
    setDepartments(getDepartments())
    onUpdate?.()
  }

  const handleAdd = () => {
    if (!newName.trim()) {
      message.warning('请输入部门名称')
      return
    }
    if (!newPath.trim()) {
      message.warning('请输入归档路径')
      return
    }
    addDepartment(newName.trim(), newPath.trim())
    setNewName('')
    setNewPath('')
    setAdding(false)
    refreshDepartments()
    message.success('部门添加成功')
  }

  const handleEdit = (dept) => {
    setEditingId(dept.id)
    setEditName(dept.name)
    setEditPath(dept.archivePath)
  }

  const handleSaveEdit = () => {
    if (!editName.trim()) {
      message.warning('部门名称不能为空')
      return
    }
    if (!editPath.trim()) {
      message.warning('归档路径不能为空')
      return
    }
    updateDepartment(editingId, { name: editName.trim(), archivePath: editPath.trim() })
    setEditingId(null)
    refreshDepartments()
    message.success('部门更新成功')
  }

  const handleDelete = (id) => {
    deleteDepartment(id)
    refreshDepartments()
    message.success('部门已删除')
  }

  const handleSetDefault = (id) => {
    setDefaultDepartment(id)
    refreshDepartments()
    message.success('已设为默认部门')
  }

  const handleSelectFolder = async (callback) => {
    if (window.electronAPI?.selectFolder) {
      const selectedPath = await window.electronAPI.selectFolder()
      if (selectedPath) {
        callback(selectedPath)
      }
    } else {
      message.info('请手动输入路径，或在 Electron 应用中使用文件夹选择')
    }
  }

  return (
    <div className="department-manager">
      <div className="dept-header">
        <span className="dept-title">部门列表</span>
        {!adding && (
          <Button
            type="primary"
            size="small"
            icon={<PlusOutlined />}
            onClick={() => setAdding(true)}
          >
            添加部门
          </Button>
        )}
      </div>

      {adding && (
        <div className="dept-add-form">
          <Input
            placeholder="部门名称"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            style={{ marginBottom: 8 }}
          />
          <Space.Compact style={{ width: '100%', marginBottom: 8 }}>
            <Input
              prefix={<FolderOutlined />}
              placeholder="归档路径，如 D:/归档/科数部"
              value={newPath}
              onChange={(e) => setNewPath(e.target.value)}
            />
            <Button onClick={() => handleSelectFolder(setNewPath)}>选择</Button>
          </Space.Compact>
          <Space>
            <Button type="primary" size="small" icon={<CheckOutlined />} onClick={handleAdd}>
              确定
            </Button>
            <Button size="small" icon={<CloseOutlined />} onClick={() => {
              setAdding(false)
              setNewName('')
              setNewPath('')
            }}>
              取消
            </Button>
          </Space>
        </div>
      )}

      {departments.length === 0 && !adding ? (
        <Empty description="暂无部门，请添加" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <List
          className="dept-list"
          dataSource={departments}
          renderItem={(dept) => (
            <List.Item className="dept-item">
              {editingId === dept.id ? (
                <div className="dept-edit-form">
                  <Input
                    placeholder="部门名称"
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
                      {dept.name}
                      {defaultDeptId === dept.id && (
                        <Tag color="gold" style={{ marginLeft: 8 }}>默认</Tag>
                      )}
                    </span>
                    <span className="dept-path">{dept.archivePath}</span>
                  </div>
                  <Space className="dept-actions">
                    <Button
                      type="text"
                      size="small"
                      icon={defaultDeptId === dept.id ? <StarFilled style={{ color: '#faad14' }} /> : <StarOutlined />}
                      onClick={() => handleSetDefault(dept.id)}
                      title="设为默认"
                    />
                    <Button
                      type="text"
                      size="small"
                      icon={<EditOutlined />}
                      onClick={() => handleEdit(dept)}
                    />
                    <Popconfirm
                      title="确定删除此部门？"
                      onConfirm={() => handleDelete(dept.id)}
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

export default DepartmentManager
