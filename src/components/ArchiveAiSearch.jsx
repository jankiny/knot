import { useMemo, useState } from 'react'
import { Alert, Button, Card, Empty, Input, List, message, Select, Space, Tag, Tooltip } from 'antd'
import { FolderOpenOutlined, RobotOutlined, SearchOutlined } from '@ant-design/icons'
import { archiveAiApi } from '../services/api'
import { getDepartments, getProjects } from '../services/settings'
import { getReportAiConfig } from '../hooks/useAiConfig'
import './ArchiveAiSearch.css'

function getArchiveSearchTargets() {
  const departments = getDepartments().map((item) => ({
    ...item,
    key: `department:${item.id}`,
    type: 'department',
    typeLabel: '部门'
  }))
  const projects = getProjects().map((item) => ({
    ...item,
    key: `project:${item.id}`,
    type: 'project',
    typeLabel: '项目'
  }))
  return [...departments, ...projects].filter((item) => item.archivePath)
}

function ArchiveAiSearch() {
  const [targets] = useState(() => getArchiveSearchTargets())
  const [selectedTargetKeys, setSelectedTargetKeys] = useState(() => getArchiveSearchTargets().map((item) => item.key))
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(false)
  const [matches, setMatches] = useState([])
  const [candidateCount, setCandidateCount] = useState(0)
  const [suggestedQuery, setSuggestedQuery] = useState('')

  const selectedTargets = useMemo(
    () => targets.filter((item) => selectedTargetKeys.includes(item.key)),
    [targets, selectedTargetKeys]
  )

  const targetOptions = targets.map((target) => ({
    label: `${target.name}（${target.typeLabel}）`,
    value: target.key,
    desc: target.archivePath,
    type: target.type
  }))

  const handleSearch = async () => {
    const trimmedQuery = query.trim()
    if (!trimmedQuery) {
      message.warning('请输入要查找的资料线索')
      return
    }
    if (selectedTargets.length === 0) {
      message.warning('请至少选择一个归档范围')
      return
    }

    setLoading(true)
    setMatches([])
    setSuggestedQuery('')
    setCandidateCount(0)
    try {
      const aiConfig = await getReportAiConfig({
        featureLabel: 'AI 归档检索'
      })
      if (!aiConfig) return

      const resp = await archiveAiApi.search({
        query: trimmedQuery,
        archive_paths: selectedTargets.map((item) => item.archivePath),
        limit: 8,
        ai: aiConfig
      })
      setMatches(resp.matches || [])
      setCandidateCount(resp.candidate_count || 0)
      setSuggestedQuery(resp.suggested_query || '')
      if ((resp.matches || []).length === 0) {
        message.info('AI 未找到明确匹配的归档项目')
      }
    } catch (error) {
      message.error(error.response?.data?.detail || 'AI 归档检索失败')
    } finally {
      setLoading(false)
    }
  }

  const handleOpenFolder = async (folderPath) => {
    if (!window.electronAPI?.openFolder) {
      message.info('请在 Electron 客户端中打开目录')
      return
    }
    const ok = await window.electronAPI.openFolder(folderPath)
    if (!ok) message.error('打开目录失败')
  }

  return (
    <div className="archive-ai-search">
      <Card title="AI 归档检索" className="archive-ai-card">
        <Alert
          className="archive-ai-notice"
          type="info"
          showIcon
          message="AI 会读取候选归档项目的标题、路径和工作记录摘要，用于判断最可能相关的资料位置。"
        />

        <div className="archive-ai-controls">
          <Select
            mode="multiple"
            className="archive-ai-target-select"
            placeholder="选择归档范围"
            value={selectedTargetKeys}
            onChange={setSelectedTargetKeys}
            options={targetOptions}
            optionRender={(option) => (
              <div className="archive-ai-target-option">
                <Space size={6}>
                  <span>{option.data.label}</span>
                  <Tag color={option.data.type === 'project' ? 'purple' : 'blue'}>{option.data.type === 'project' ? '项目' : '部门'}</Tag>
                </Space>
                <span>{option.data.desc}</span>
              </div>
            )}
          />
          <Input.Search
            className="archive-ai-query"
            prefix={<SearchOutlined />}
            placeholder="例如：样本库数据处理、去年周报模板、科数部培训资料"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onSearch={handleSearch}
            enterButton={
              <Button type="primary" icon={<RobotOutlined />} loading={loading}>
                AI 检索
              </Button>
            }
            loading={loading}
            allowClear
          />
        </div>
      </Card>

      <Card
        title="检索结果"
        className="archive-ai-card"
        extra={candidateCount > 0 ? <Tag color="blue">候选项目 {candidateCount}</Tag> : null}
      >
        {targets.length === 0 ? (
          <Empty description="暂无归档范围，请先在设置中配置部门或项目归档目录" />
        ) : matches.length === 0 ? (
          <Empty description={loading ? 'AI 正在检索归档项目...' : '输入线索后开始检索'} image={Empty.PRESENTED_IMAGE_SIMPLE} />
        ) : (
          <List
            className="archive-ai-result-list"
            dataSource={matches}
            renderItem={(item) => (
              <List.Item
                actions={[
                  <Tooltip title="打开归档目录" key="open">
                    <Button icon={<FolderOpenOutlined />} onClick={() => handleOpenFolder(item.path)}>
                      打开
                    </Button>
                  </Tooltip>
                ]}
              >
                <List.Item.Meta
                  title={
                    <div className="archive-ai-result-title">
                      <span>{item.title || item.folder_name || '未命名归档项目'}</span>
                      {item.confidence !== undefined && (
                        <Tag color={item.confidence >= 0.75 ? 'green' : item.confidence >= 0.45 ? 'orange' : 'default'}>
                          置信度 {Math.round((item.confidence || 0) * 100)}%
                        </Tag>
                      )}
                    </div>
                  }
                  description={
                    <div className="archive-ai-result-body">
                      <div className="archive-ai-tags">
                        {item.task_date && <Tag>{item.task_date}</Tag>}
                        {item.department && <Tag color="blue">{item.department}</Tag>}
                        {item.project && <Tag color="purple">{item.project}</Tag>}
                        {(item.matched_keywords || []).map((keyword) => (
                          <Tag key={`${item.path}-${keyword}`} color="cyan">{keyword}</Tag>
                        ))}
                      </div>
                      {item.reason && <div className="archive-ai-reason">{item.reason}</div>}
                      <div className="archive-ai-path">{item.path}</div>
                    </div>
                  }
                />
              </List.Item>
            )}
          />
        )}

        {suggestedQuery && (
          <Alert
            className="archive-ai-suggestion"
            type="warning"
            showIcon
            message={suggestedQuery}
          />
        )}
      </Card>
    </div>
  )
}

export default ArchiveAiSearch
