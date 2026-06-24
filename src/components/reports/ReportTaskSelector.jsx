import { Button, Card, Checkbox, Empty, Input, List, Space, Tag } from 'antd'

function ReportTaskSelector({
  folders,
  getDateLabel,
  searchText,
  selectedFolderPaths,
  setAllFolderChecked,
  setSearchText,
  setSelectedFolderPaths
}) {
  return (
    <Card title="任务筛选" className="daily-card">
      <div className="task-toolbar">
        <Input
          value={searchText}
          onChange={(event) => setSearchText(event.target.value)}
          placeholder="搜索任务名称 / 路径 / 归属"
          allowClear
        />
        <Space>
          <Button onClick={() => setAllFolderChecked(true)}>全选当前列表</Button>
          <Button onClick={() => setAllFolderChecked(false)}>取消当前列表</Button>
        </Space>
      </div>

      {folders.length === 0 ? (
        <Empty description="请先扫描任务目录" />
      ) : (
        <List
          dataSource={folders}
          renderItem={(folder) => {
            const dateLabel = getDateLabel?.(folder)
            return (
              <List.Item>
                <div className="task-item">
                  <Checkbox
                    checked={!!selectedFolderPaths[folder.path]}
                    onChange={(event) => setSelectedFolderPaths((prev) => ({ ...prev, [folder.path]: event.target.checked }))}
                  >
                    <span className="task-title">{folder.title || folder.name}</span>
                  </Checkbox>
                  <div className="task-meta">
                    {folder.department && <Tag color="blue">{folder.department}</Tag>}
                    {folder.project && <Tag color="purple">{folder.project}</Tag>}
                    {dateLabel && <Tag>{dateLabel}</Tag>}
                    {(folder.fromDirectories || []).map((dirName) => (
                      <Tag key={`${folder.path}-${dirName}`} color="blue">{dirName}</Tag>
                    ))}
                  </div>
                </div>
              </List.Item>
            )
          }}
        />
      )}
    </Card>
  )
}

export default ReportTaskSelector
