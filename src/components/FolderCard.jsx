import { Card, Tag, Button, Tooltip, Space } from 'antd'
import { FolderOutlined, FileTextOutlined, ClockCircleOutlined, SendOutlined, EditOutlined, EyeOutlined } from '@ant-design/icons'
import './FolderCard.css'

function FolderCard({ folder, departments, onArchive, onEditDept, onViewContent }) {
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

  const hasDept = folder.department && folder.department.trim() !== ''

  return (
    <Card className="folder-card" size="small">
      <div className="folder-card-content">
        <div className="folder-info">
          <div className="folder-name">
            <FolderOutlined className="folder-icon" />
            <span>{folder.name}</span>
          </div>

          <div className="folder-meta">
            {folder.create_time && (
              <Tooltip title="创建时间">
                <span className="meta-item">
                  <ClockCircleOutlined />
                  {folder.create_time}
                </span>
              </Tooltip>
            )}

            {folder.source && (
              <Tag color="blue" style={{ marginLeft: 4 }}>{folder.source}</Tag>
            )}

            {hasDept ? (
              <Tag color="green" style={{ marginLeft: 4 }}>{folder.department}</Tag>
            ) : (
              <Tag color="default" style={{ marginLeft: 4 }}>未指定部门</Tag>
            )}

            {folder.file_count > 0 && (
              <span className="meta-item" style={{ marginLeft: 4 }}>
                <FileTextOutlined />
                {folder.file_count} 个文件
              </span>
            )}
          </div>

          {folder.content && (
            <div className="folder-content-preview">
              {folder.content.length > 80 ? folder.content.substring(0, 80) + '...' : folder.content}
            </div>
          )}
        </div>

        <div className="folder-actions">
          <Space>
            <Tooltip title="查看/编辑工作记录">
              <Button
                size="small"
                icon={<EyeOutlined />}
                onClick={() => onViewContent(folder)}
              >
                记录
              </Button>
            </Tooltip>

            <Tooltip title="编辑归属部门">
              <Button
                size="small"
                icon={<EditOutlined />}
                onClick={() => onEditDept(folder)}
              >
                部门
              </Button>
            </Tooltip>

            <Tooltip title={hasDept ? '归档到部门目录' : '请先指定部门'}>
              <Button
                size="small"
                type="primary"
                icon={<SendOutlined />}
                onClick={() => onArchive(folder)}
                disabled={!hasDept}
              >
                归档
              </Button>
            </Tooltip>
          </Space>
        </div>
      </div>
    </Card>
  )
}

export default FolderCard
