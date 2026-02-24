import { Card, Tag, Checkbox, Select, Tooltip } from 'antd'
import { FolderOutlined, FileMarkdownOutlined, ClockCircleOutlined } from '@ant-design/icons'
import './FolderCard.css'

function FolderCard({ folder, departments, selected, onSelect, selectedDeptId, onDeptChange }) {
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

  // 根据工作记录中的部门自动匹配
  const matchedDept = folder.department
    ? departments.find(d => d.name === folder.department)
    : null

  const currentDeptId = selectedDeptId || (matchedDept ? matchedDept.id : null)

  return (
    <Card className={`folder-card ${selected ? 'selected' : ''}`} size="small">
      <div className="folder-card-content">
        <Checkbox
          checked={selected}
          onChange={(e) => onSelect(folder.path, e.target.checked)}
          className="folder-checkbox"
        />

        <div className="folder-info">
          <div className="folder-name">
            <FolderOutlined className="folder-icon" />
            <span>{folder.name}</span>
          </div>

          <div className="folder-meta">
            <Tooltip title="最后修改时间">
              <span className="meta-item">
                <ClockCircleOutlined />
                {formatDate(folder.modified)}
              </span>
            </Tooltip>

            {folder.has_work_record && (
              <Tooltip title={`工作记录：${folder.department || '未指定部门'}`}>
                <Tag icon={<FileMarkdownOutlined />} color="green">
                  {folder.department || '有工作记录'}
                </Tag>
              </Tooltip>
            )}

            {!folder.has_work_record && (
              <Tag color="default">无工作记录</Tag>
            )}
          </div>
        </div>

        <div className="folder-dept-select">
          <Select
            size="small"
            placeholder="选择部门"
            value={currentDeptId}
            onChange={(value) => onDeptChange(folder.path, value)}
            options={departments.map(d => ({
              label: d.name,
              value: d.id
            }))}
            style={{ width: 120 }}
            allowClear
          />
        </div>
      </div>
    </Card>
  )
}

export default FolderCard
