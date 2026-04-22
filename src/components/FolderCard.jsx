import { Button, Card, Space, Tag, Tooltip } from 'antd'
import { ClockCircleOutlined, EditOutlined, FileTextOutlined, FolderOutlined, SendOutlined } from '@ant-design/icons'
import './FolderCard.css'

const SOURCE_LABEL_MAP = {
  manual: '手动',
  email: '邮件'
}

function getSourceLabel(source) {
  const key = String(source || '').trim().toLowerCase()
  return SOURCE_LABEL_MAP[key] || source || '未知'
}

function FolderCard({ folder, onArchive, onEditDept, onViewContent, onOpenFolder }) {
  const title = folder.title || folder.name
  const hasDept = !!(folder.department && folder.department.trim())
  const departmentLabel = hasDept ? folder.department : '未指定部门'

  return (
    <Card className="folder-card" size="small">
      <div className="folder-card-content">
        <div className="folder-row folder-row-top">
          <div className="folder-title-wrap" onClick={() => onOpenFolder?.(folder)} role="button" tabIndex={0}>
            <Tooltip title={`打开文件夹：${folder.path || ''}`}>
              <FolderOutlined className="folder-icon" />
            </Tooltip>
            <Tooltip title={title}>
              <span className="folder-title">{title}</span>
            </Tooltip>
          </div>

          <div className="folder-actions">
            <Space size={8}>
              <Button size="small" icon={<EditOutlined />} onClick={() => onViewContent(folder)}>
                记录
              </Button>
              <Button
                size="small"
                type="primary"
                icon={<SendOutlined />}
                onClick={() => onArchive(folder)}
                disabled={!hasDept}
              >
                归档
              </Button>
            </Space>
          </div>
        </div>

        <div className="folder-row folder-row-meta">
          {folder.create_time && (
            <Tag icon={<ClockCircleOutlined />}>
              {folder.create_time}
            </Tag>
          )}

          {folder.source && (
            <Tag color="blue">
              {getSourceLabel(folder.source)}
            </Tag>
          )}

          <Tooltip title="点击编辑所属部门">
            <Tag
              color={hasDept ? 'green' : 'default'}
              className="folder-department-tag"
              onClick={() => onEditDept(folder)}
            >
              {departmentLabel}
            </Tag>
          </Tooltip>

          {Number(folder.file_count) > 0 && (
            <Tag icon={<FileTextOutlined />}>
              {folder.file_count} 个文件
            </Tag>
          )}
        </div>

        {folder.content && (
          <div className="folder-content-preview">
            {folder.content.length > 200 ? `${folder.content.slice(0, 200)}...` : folder.content}
          </div>
        )}
      </div>
    </Card>
  )
}

export default FolderCard
