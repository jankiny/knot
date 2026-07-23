import { useCallback, useEffect, useState } from 'react'
import {
  Button,
  Form,
  Input,
  message,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip
} from 'antd'
import {
  DeleteOutlined,
  EditOutlined,
  FolderOpenOutlined,
  PlusOutlined,
  ReloadOutlined
} from '@ant-design/icons'
import { sourceRootApi } from '../../services/api'
import { importLegacySourceRoots, sourceRootToInput } from '../../services/sourceRoots'

const KIND_OPTIONS = [
  { value: 'current_work', label: '当前工作' },
  { value: 'active_work', label: '扫描工作' },
  { value: 'work_archive', label: '工作归档' },
  { value: 'journal', label: '工作日志' },
  { value: 'reference', label: '参考资料' },
  { value: 'private', label: '私有资料' },
  { value: 'external_offline', label: '外部离线资料' }
]

const SCOPE_OPTIONS = [
  { value: 'global', label: '全局' },
  { value: 'department', label: '部门' },
  { value: 'project', label: '项目' }
]

const AVAILABILITY = {
  online: { color: 'success', label: '在线' },
  offline: { color: 'default', label: '离线' },
  missing: { color: 'warning', label: '缺失' },
  permission_denied: { color: 'error', label: '无权限' },
  unknown: { color: 'default', label: '未知' }
}

const KIND_LABELS = Object.fromEntries(KIND_OPTIONS.map((item) => [item.value, item.label]))
const SCOPE_LABELS = Object.fromEntries(SCOPE_OPTIONS.map((item) => [item.value, item.label]))

function SourceRootsSettingsSection({ legacySettings }) {
  const [form] = Form.useForm()
  const [roots, setRoots] = useState([])
  const [loading, setLoading] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [editingRoot, setEditingRoot] = useState(null)
  const [modalOpen, setModalOpen] = useState(false)
  const scopeType = Form.useWatch('scope_type', form)

  const loadRoots = useCallback(async ({ quiet = false } = {}) => {
    setLoading(true)
    try {
      const response = await sourceRootApi.list()
      setRoots(response.source_roots || [])
    } catch (error) {
      if (!quiet) {
        message.error(error?.response?.data?.detail || '读取资料源失败')
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadRoots({ quiet: true })
  }, [loadRoots])

  const openCreate = (kind = 'reference') => {
    setEditingRoot(null)
    form.setFieldsValue({
      name: kind === 'journal' ? '工作日志' : '',
      kind,
      path: '',
      scope_type: 'global',
      scope_id: '',
      scope_name: '',
      enabled: true,
      recursive: true,
      local_access: 'read',
      ai_access: kind === 'private' ? 'none' : 'content'
    })
    setModalOpen(true)
  }

  const openEdit = (root) => {
    setEditingRoot(root)
    form.setFieldsValue(sourceRootToInput(root))
    setModalOpen(true)
  }

  const closeModal = () => {
    setModalOpen(false)
    setEditingRoot(null)
    form.resetFields()
  }

  const chooseFolder = async () => {
    if (!window.electronAPI?.selectFolder) {
      message.info('请在 Electron 客户端中选择目录')
      return
    }
    const path = await window.electronAPI.selectFolder()
    if (path) {
      form.setFieldValue('path', path)
    }
  }

  const saveRoot = async () => {
    let values
    try {
      values = await form.validateFields()
    } catch {
      return
    }

    const payload = sourceRootToInput(values)
    setSaving(true)
    try {
      if (editingRoot) {
        await sourceRootApi.update(editingRoot.id, payload)
        message.success('资料源已更新')
      } else {
        await sourceRootApi.create(payload)
        message.success('资料源已添加')
      }
      closeModal()
      await loadRoots({ quiet: true })
    } catch (error) {
      message.error(error?.response?.data?.detail || '保存资料源失败')
    } finally {
      setSaving(false)
    }
  }

  const deleteRoot = async (root) => {
    try {
      await sourceRootApi.remove(root.id)
      message.success('资料源已删除')
      await loadRoots({ quiet: true })
    } catch (error) {
      message.error(error?.response?.data?.detail || '删除资料源失败')
    }
  }

  const syncLegacySettings = async () => {
    setSyncing(true)
    try {
      const result = await importLegacySourceRoots(legacySettings)
      await loadRoots({ quiet: true })
      message.success(`同步完成，新增 ${result.imported?.length || 0} 个资料源`)
    } catch (error) {
      message.error(error?.response?.data?.detail || '同步现有目录设置失败')
    } finally {
      setSyncing(false)
    }
  }

  const columns = [
    {
      title: '资料源',
      dataIndex: 'name',
      width: 150,
      render: (name, root) => (
        <div className="source-root-name">
          <span>{name}</span>
          <span>{KIND_LABELS[root.kind] || root.kind}</span>
        </div>
      )
    },
    {
      title: '目录',
      dataIndex: 'path',
      ellipsis: true,
      render: (path) => <Tooltip title={path}>{path}</Tooltip>
    },
    {
      title: '范围',
      dataIndex: 'scope_type',
      width: 115,
      render: (scopeTypeValue, root) => (
        root.scope_name || SCOPE_LABELS[scopeTypeValue] || scopeTypeValue
      )
    },
    {
      title: '可用性',
      dataIndex: 'availability',
      width: 85,
      render: (availability) => {
        const status = AVAILABILITY[availability] || AVAILABILITY.unknown
        return <Tag color={status.color}>{status.label}</Tag>
      }
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      width: 75,
      render: (enabled) => (
        <Tag color={enabled ? 'blue' : 'default'}>{enabled ? '启用' : '停用'}</Tag>
      )
    },
    {
      title: '操作',
      key: 'actions',
      width: 92,
      render: (_, root) => (
        <Space size={2}>
          <Button
            type="text"
            size="small"
            icon={<EditOutlined />}
            aria-label={`编辑${root.name}`}
            onClick={() => openEdit(root)}
          />
          <Popconfirm
            title="删除资料源？"
            description="只删除注册记录，不会删除目录或文件。"
            okText="删除"
            cancelText="取消"
            onConfirm={() => deleteRoot(root)}
          >
            <Button
              type="text"
              danger
              size="small"
              icon={<DeleteOutlined />}
              aria-label={`删除${root.name}`}
            />
          </Popconfirm>
        </Space>
      )
    }
  ]

  return (
    <div id="source-roots-settings" className="settings-block">
      <div className="source-roots-heading">
        <div>
          <h2>资料源</h2>
          <p className="section-desc">
            Knot 只登记目录和访问策略；本阶段不会扫描文件、读取正文或调用 AI。
          </p>
        </div>
        <Space>
          <Button loading={syncing} onClick={syncLegacySettings}>
            同步现有设置
          </Button>
          <Button icon={<ReloadOutlined />} onClick={() => loadRoots()}>
            刷新
          </Button>
          <Button icon={<PlusOutlined />} onClick={() => openCreate('reference')}>
            添加资料源
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => openCreate('journal')}>
            添加工作日志
          </Button>
        </Space>
      </div>

      <Table
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={roots}
        pagination={false}
        locale={{ emptyText: '暂无资料源；现有工作和归档设置会自动导入。' }}
        scroll={{ x: 760 }}
      />

      <Modal
        title={editingRoot ? '编辑资料源' : '添加资料源'}
        open={modalOpen}
        okText={editingRoot ? '保存' : '添加'}
        cancelText="取消"
        confirmLoading={saving}
        onOk={saveRoot}
        onCancel={closeModal}
        forceRender
      >
        <Form form={form} layout="vertical" className="source-root-form">
          <Form.Item
            label="名称"
            name="name"
            rules={[{ required: true, whitespace: true, message: '请输入资料源名称' }]}
          >
            <Input placeholder="例如：工作日志" />
          </Form.Item>

          <Form.Item
            label="类型"
            name="kind"
            rules={[{ required: true, message: '请选择资料源类型' }]}
          >
            <Select options={KIND_OPTIONS} />
          </Form.Item>

          <Form.Item
            label="目录"
            name="path"
            rules={[{ required: true, whitespace: true, message: '请选择或输入绝对目录' }]}
          >
            <Space.Compact style={{ width: '100%' }}>
              <Input placeholder="例如：C:\Workspace\Journal" />
              <Button icon={<FolderOpenOutlined />} onClick={chooseFolder}>选择</Button>
            </Space.Compact>
          </Form.Item>

          <Form.Item label="归属范围" name="scope_type">
            <Select options={SCOPE_OPTIONS} />
          </Form.Item>

          {scopeType !== 'global' && (
            <div className="source-root-scope-fields">
              <Form.Item label="归属 ID" name="scope_id">
                <Input placeholder="用于稳定关联部门或项目" />
              </Form.Item>
              <Form.Item label="归属名称" name="scope_name">
                <Input placeholder="例如：办公室" />
              </Form.Item>
            </div>
          )}

          <div className="source-root-policy-fields">
            <Form.Item label="本地访问" name="local_access">
              <Select
                options={[
                  { value: 'none', label: '不访问' },
                  { value: 'read', label: '只读' },
                  { value: 'read_write', label: '读写' }
                ]}
              />
            </Form.Item>
            <Form.Item label="AI 暴露" name="ai_access">
              <Select
                options={[
                  { value: 'none', label: '禁止' },
                  { value: 'metadata', label: '仅元数据' },
                  { value: 'content', label: '允许内容' }
                ]}
              />
            </Form.Item>
          </div>

          <div className="source-root-switches">
            <Form.Item label="启用" name="enabled" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item label="递归包含子目录" name="recursive" valuePropName="checked">
              <Switch />
            </Form.Item>
          </div>
        </Form>
      </Modal>
    </div>
  )
}

export default SourceRootsSettingsSection
