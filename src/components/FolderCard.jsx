import { Button, Card, Space, Tag, Tooltip } from 'antd'
import {
  ClockCircleOutlined,
  EditOutlined,
  FileTextOutlined,
  FolderOutlined,
  SendOutlined
} from '@ant-design/icons'
import './FolderCard.css'

const SOURCE_LABEL_MAP = {
  manual: '\u624b\u52a8',
  email: '\u90ae\u4ef6'
}

const LEGACY_TITLE_SET = new Set([
  '\u5de5\u4f5c\u8bb0\u5f55',
  '\u5de5\u4f5c.md',
  '\u5de5\u4f5c'
])

const UNKNOWN_LABEL = '\u672a\u77e5'
const UNSET_DEPARTMENT_LABEL = '\u672a\u6307\u5b9a\u90e8\u95e8'
const BTN_RECORD = '\u8bb0\u5f55'
const BTN_ARCHIVE = '\u5f52\u6863'
const OPEN_FOLDER_PREFIX = '\u6253\u5f00\u6587\u4ef6\u5939\uff1a'
const EDIT_DEPT_TIP = '\u70b9\u51fb\u7f16\u8f91\u6240\u5c5e\u90e8\u95e8'
const FILE_UNIT = '\u4e2a\u6587\u4ef6'

export function getDisplayTitle(folder) {
  const title = String(folder?.title || '').trim()
  if (!title || LEGACY_TITLE_SET.has(title)) {
    return folder?.name || ''
  }
  return title
}

function getSourceLabel(source) {
  const key = String(source || '').trim().toLowerCase()
  return SOURCE_LABEL_MAP[key] || source || UNKNOWN_LABEL
}

function FolderCard({ folder, onArchive, onEditDept, onViewContent, onOpenFolder }) {
  const title = getDisplayTitle(folder)
  const hasDept = Boolean(folder?.department && folder.department.trim())
  const departmentLabel = hasDept ? folder.department : UNSET_DEPARTMENT_LABEL

  return (
    <Card className="folder-card" size="small">
      <div className="folder-card-content">
        <div className="folder-row folder-row-top">
          <div
            className="folder-title-wrap"
            onClick={() => onOpenFolder?.(folder)}
            role="button"
            tabIndex={0}
          >
            <Tooltip title={`${OPEN_FOLDER_PREFIX}${folder.path || ''}`}>
              <FolderOutlined className="folder-icon" />
            </Tooltip>
            <Tooltip title={title}>
              <span className="folder-title">{title}</span>
            </Tooltip>
          </div>

          <div className="folder-actions">
            <Space size={8}>
              <Button size="small" icon={<EditOutlined />} onClick={() => onViewContent(folder)}>
                {BTN_RECORD}
              </Button>
              <Button
                size="small"
                type="primary"
                icon={<SendOutlined />}
                onClick={() => onArchive(folder)}
                disabled={!hasDept}
              >
                {BTN_ARCHIVE}
              </Button>
            </Space>
          </div>
        </div>

        <div className="folder-row folder-row-meta">
          {folder.create_time && <Tag icon={<ClockCircleOutlined />}>{folder.create_time}</Tag>}
          {folder.source && <Tag color="blue">{getSourceLabel(folder.source)}</Tag>}
          <Tooltip title={EDIT_DEPT_TIP}>
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
              {folder.file_count} {FILE_UNIT}
            </Tag>
          )}
        </div>

        {folder.content && <div className="folder-content-preview">{folder.content}</div>}
      </div>
    </Card>
  )
}

export default FolderCard
