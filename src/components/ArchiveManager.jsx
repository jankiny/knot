import { useEffect, useMemo, useState } from 'react'
import { Button, Empty, Input, message, Select, Space, Spin, Tag, Tooltip } from 'antd'
import { InboxOutlined, ReloadOutlined, RollbackOutlined, SearchOutlined } from '@ant-design/icons'
import { archiveApi } from '../services/api'
import { getDepartments, getProjects, getSettings } from '../services/settings'
import FolderCard from './FolderCard'
import './ArchiveManager.css'

function getArchiveTargets() {
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

function ArchiveManager() {
  const [targets, setTargets] = useState([])
  const [targetKey, setTargetKey] = useState('')
  const [folders, setFolders] = useState([])
  const [years, setYears] = useState([])
  const [year, setYear] = useState('')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [hasMore, setHasMore] = useState(false)
  const [loading, setLoading] = useState(false)
  const [restoring, setRestoring] = useState({})

  const selectedTarget = useMemo(
    () => targets.find((item) => item.key === targetKey) || null,
    [targets, targetKey]
  )

  useEffect(() => {
    const nextTargets = getArchiveTargets()
    setTargets(nextTargets)
    if (nextTargets.length > 0) {
      setTargetKey(nextTargets[0].key)
    }
  }, [])

  const loadArchives = async ({ nextPage = 1, append = false } = {}) => {
    if (!selectedTarget?.archivePath) return
    setLoading(true)
    try {
      const result = await archiveApi.list({
        archivePath: selectedTarget.archivePath,
        page: nextPage,
        pageSize: 30,
        keyword,
        year
      })
      setFolders((prev) => append ? [...prev, ...(result.folders || [])] : (result.folders || []))
      setYears(result.years || [])
      setPage(nextPage)
      setHasMore(!!result.has_more)
    } catch (error) {
      message.error(error.response?.data?.detail || '加载归档失败')
      if (!append) setFolders([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (selectedTarget) {
      loadArchives({ nextPage: 1, append: false })
    }
  }, [selectedTarget, year])

  const handleSearch = () => {
    loadArchives({ nextPage: 1, append: false })
  }

  const handleRestore = async (folder) => {
    const settings = getSettings()
    const restorePath = settings.folderPath || settings.scanPath || '~/Desktop'
    setRestoring((prev) => ({ ...prev, [folder.path]: true }))
    try {
      const result = await archiveApi.restore(folder.path, restorePath)
      message.success(result.message || '已恢复到当前工作')
      setFolders((prev) => prev.filter((item) => item.path !== folder.path))
    } catch (error) {
      message.error(error.response?.data?.detail || '恢复失败')
    } finally {
      setRestoring((prev) => ({ ...prev, [folder.path]: false }))
    }
  }

  const handleOpenFolder = async (folder) => {
    if (!window.electronAPI?.openFolder) {
      message.info('请在 Electron 客户端中打开目录')
      return
    }
    const ok = await window.electronAPI.openFolder(folder.path)
    if (!ok) message.error('打开目录失败')
  }

  const targetOptions = targets.map((target) => ({
    label: target.name,
    value: target.key,
    desc: target.archivePath,
    type: target.type,
    typeLabel: target.typeLabel
  }))

  return (
    <div className="archive-manager">
      <div className="archive-manager-toolbar">
        <Select
          className="archive-target-select"
          placeholder="选择归档目标"
          value={targetKey || undefined}
          onChange={(value) => {
            setTargetKey(value)
            setFolders([])
            setYear('')
            setKeyword('')
          }}
          options={targetOptions}
          optionRender={(option) => (
            <div className="archive-target-option">
              <Space size={6}>
                <span>{option.data.label}</span>
                <Tag color={option.data.type === 'project' ? 'purple' : 'blue'}>{option.data.typeLabel}</Tag>
              </Space>
              <span>{option.data.desc}</span>
            </div>
          )}
        />

        <Select
          className="archive-year-select"
          placeholder="全部年份"
          value={year || undefined}
          allowClear
          onChange={(value) => setYear(value || '')}
          options={years.map((item) => ({ label: item, value: item }))}
        />

        <Input
          className="archive-search-input"
          prefix={<SearchOutlined />}
          placeholder="搜索标题、内容或归属"
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          onPressEnter={handleSearch}
          allowClear
        />

        <Tooltip title="刷新归档列表">
          <Button icon={<ReloadOutlined />} onClick={() => loadArchives({ nextPage: 1, append: false })} loading={loading} />
        </Tooltip>
      </div>

      {!targetOptions.length ? (
        <Empty description="暂无归档目标，请先在设置中配置部门或项目归档目录" />
      ) : loading && folders.length === 0 ? (
        <div className="archive-manager-loading">
          <Spin />
          <span>加载归档项目中...</span>
        </div>
      ) : folders.length === 0 ? (
        <Empty description="当前筛选条件下暂无归档项目" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <div className="archive-manager-list">
          {folders.map((folder) => (
            <div className="archive-manager-item" key={folder.path}>
              <FolderCard
                folder={folder}
                onArchive={handleRestore}
                onEditDept={() => {}}
                onViewContent={() => handleOpenFolder(folder)}
                onOpenFolder={handleOpenFolder}
                archiveLabel="恢复"
                archiveIcon={<RollbackOutlined />}
                archiveDisabled={false}
                archiveLoading={!!restoring[folder.path]}
              />
            </div>
          ))}
          {hasMore && (
            <Button
              className="load-more-btn"
              icon={<InboxOutlined />}
              onClick={() => loadArchives({ nextPage: page + 1, append: true })}
              loading={loading}
            >
              加载更多
            </Button>
          )}
        </div>
      )}
    </div>
  )
}

export default ArchiveManager
