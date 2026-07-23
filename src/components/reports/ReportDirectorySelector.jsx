import { Button, Card, Checkbox, Empty, Typography } from 'antd'
import { FolderOpenOutlined, ReloadOutlined } from '@ant-design/icons'

const { Text } = Typography

function ReportDirectorySelector({
  title,
  directories,
  loading,
  onAddDirectory,
  onRemoveDirectory,
  onScan,
  onToggleDirectory
}) {
  return (
    <Card title={title} className="daily-card">
      <div className="directory-actions">
        <Button onClick={onAddDirectory} icon={<FolderOpenOutlined />}>添加目录</Button>
        <Button type="primary" onClick={onScan} icon={<ReloadOutlined />} loading={loading}>扫描任务</Button>
      </div>

      <div className="directory-list">
        {directories.length === 0 && <Empty description="暂无可用目录" />}
        {directories.map((item) => (
          <div className="directory-item" key={item.id}>
            <Checkbox checked={item.checked} onChange={(event) => onToggleDirectory(item.id, event.target.checked)}>
              <span className="directory-label">{item.label}</span>
            </Checkbox>
            <Text type="secondary" className="directory-path">{item.path}</Text>
            {!item.builtin && (
              <Button type="link" danger onClick={() => onRemoveDirectory(item.id)}>移除</Button>
            )}
          </div>
        ))}
      </div>
    </Card>
  )
}

export default ReportDirectorySelector
