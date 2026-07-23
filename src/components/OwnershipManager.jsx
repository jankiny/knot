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
import './DepartmentManager.css'

function OwnershipManager({ config, onUpdate }) {
  const [items, setItems] = useState(config.getItems())
  const [adding, setAdding] = useState(false)
  const [editingId, setEditingId] = useState(null)
  const [newName, setNewName] = useState('')
  const [newPath, setNewPath] = useState('')
  const [editName, setEditName] = useState('')
  const [editPath, setEditPath] = useState('')

  const defaultItemId = config.getDefaultId()

  const refreshItems = () => {
    setItems(config.getItems())
    onUpdate?.()
  }

  const handleAdd = () => {
    if (!newName.trim()) {
      message.warning(`请输入${config.label}名称`)
      return
    }
    if (!newPath.trim()) {
      message.warning('请输入归档路径')
      return
    }
    config.addItem(newName.trim(), newPath.trim())
    setNewName('')
    setNewPath('')
    setAdding(false)
    refreshItems()
    message.success(`${config.label}添加成功`)
  }

  const handleEdit = (item) => {
    setEditingId(item.id)
    setEditName(item.name)
    setEditPath(item.archivePath)
  }

  const handleSaveEdit = () => {
    if (!editName.trim()) {
      message.warning(`${config.label}名称不能为空`)
      return
    }
    if (!editPath.trim()) {
      message.warning('归档路径不能为空')
      return
    }
    config.updateItem(editingId, {
      name: editName.trim(),
      archivePath: editPath.trim(),
      useYearFolder: config.useYearFolder
    })
    setEditingId(null)
    refreshItems()
    message.success(`${config.label}更新成功`)
  }

  const handleDelete = (id) => {
    config.deleteItem(id)
    refreshItems()
    message.success(`${config.label}已删除`)
  }

  const handleSetDefault = (id) => {
    config.setDefaultItem(id)
    refreshItems()
    message.success(`已设为默认${config.label}`)
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
        <span className="dept-title">{config.listTitle}</span>
        {!adding && (
          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => setAdding(true)}>
            添加{config.label}
          </Button>
        )}
      </div>

      {adding && (
        <div className="dept-add-form">
          <Input
            placeholder={config.namePlaceholder}
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            style={{ marginBottom: 8 }}
          />
          <Space.Compact style={{ width: '100%', marginBottom: 8 }}>
            <Input
              prefix={<FolderOutlined />}
              placeholder={config.pathPlaceholder}
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

      {items.length === 0 && !adding ? (
        <Empty description={`暂无${config.label}，请添加`} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <List
          className="dept-list"
          dataSource={items}
          renderItem={(item) => (
            <List.Item className="dept-item">
              {editingId === item.id ? (
                <div className="dept-edit-form">
                  <Input
                    placeholder={`${config.label}名称`}
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
                      {item.name}
                      {defaultItemId === item.id && <Tag color="gold" style={{ marginLeft: 8 }}>默认</Tag>}
                      <Tag color={config.archiveTagColor} style={{ marginLeft: 8 }}>{config.archiveTagText}</Tag>
                    </span>
                    <span className="dept-path">{item.archivePath}</span>
                  </div>
                  <Space className="dept-actions">
                    <Button
                      type="text"
                      size="small"
                      icon={defaultItemId === item.id ? <StarFilled style={{ color: '#faad14' }} /> : <StarOutlined />}
                      onClick={() => handleSetDefault(item.id)}
                      title="设为默认"
                    />
                    <Button type="text" size="small" icon={<EditOutlined />} onClick={() => handleEdit(item)} />
                    <Popconfirm
                      title={`确定删除此${config.label}？`}
                      onConfirm={() => handleDelete(item.id)}
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

export default OwnershipManager
