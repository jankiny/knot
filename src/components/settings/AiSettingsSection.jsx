import { useState } from 'react'
import { Alert, Button, Form, Input, message, Modal, Space, Tag } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { BUILTIN_AI_MODELS, DEFAULT_AI_MODEL_ID, saveSettings } from '../../services/settings'

function AiSettingsSection({ settings, onSettingsChange }) {
  const [customModelOpen, setCustomModelOpen] = useState(false)
  const [editingAiModel, setEditingAiModel] = useState(null)
  const [modelApiKey, setModelApiKey] = useState('')
  const [customModelForm] = Form.useForm()

  const aiModels = settings.aiModels || BUILTIN_AI_MODELS
  const selectedAiModel = aiModels.find((model) => model.id === settings.aiSelectedModelId) ||
    aiModels.find((model) => model.id === DEFAULT_AI_MODEL_ID) ||
    aiModels[0]

  const updateSetting = (key, value) => {
    const newSettings = saveSettings({ [key]: value })
    onSettingsChange(newSettings)
  }

  const encryptModelApiKey = async () => {
    const key = modelApiKey.trim()
    if (!key) {
      return editingAiModel?.apiKeyEncrypted || null
    }
    if (!window.electronAPI?.encryptPassword) {
      message.error('API Key 保存失败')
      return undefined
    }
    try {
      return await window.electronAPI.encryptPassword(key)
    } catch (e) {
      console.error('加密 AI Key 失败:', e)
      message.error('API Key 保存失败')
      return undefined
    }
  }

  const openAiModelModal = async (model = null) => {
    setEditingAiModel(model)
    customModelForm.setFieldsValue({
      name: model?.name || '',
      provider: model?.provider || '',
      apiUrl: model?.apiUrl || '',
      modelId: model?.modelId || ''
    })

    let decryptedKey = ''
    if (model?.apiKeyEncrypted && window.electronAPI?.decryptPassword) {
      try {
        decryptedKey = await window.electronAPI.decryptPassword(model.apiKeyEncrypted) || ''
      } catch (e) {
        console.error('解密 AI Key 失败:', e)
      }
    }
    setModelApiKey(decryptedKey)
    setCustomModelOpen(true)
  }

  const closeAiModelModal = () => {
    setCustomModelOpen(false)
    setEditingAiModel(null)
    setModelApiKey('')
    customModelForm.resetFields()
  }

  const handleSaveAiModel = async () => {
    try {
      const values = await customModelForm.validateFields()
      const apiKeyEncrypted = await encryptModelApiKey()
      if (apiKeyEncrypted === undefined) {
        return
      }

      const modelId = values.modelId.trim()
      const id = editingAiModel?.id || `custom-${Date.now()}`
      const duplicate = aiModels.some((model) => model.id !== id && model.modelId === modelId)
      if (duplicate) {
        message.warning('该模型 ID 已存在')
        return
      }

      const nextModel = {
        id,
        name: (values.name || modelId).trim(),
        provider: (values.provider || 'custom-openai').trim(),
        apiUrl: values.apiUrl.trim().replace(/\/+$/, ''),
        modelId,
        apiKeyEncrypted,
        builtin: !!editingAiModel?.builtin
      }
      const nextModels = aiModels.map((model) => (model.id === id ? nextModel : model))
      if (!editingAiModel) {
        nextModels.push(nextModel)
      }
      const newSettings = saveSettings({
        aiModels: nextModels,
        aiSelectedModelId: id
      })
      onSettingsChange(newSettings)
      closeAiModelModal()
      message.success(editingAiModel ? '模型配置已更新' : '模型配置已添加')
    } catch {
      // antd form validation already marks invalid fields
    }
  }

  const handleDeleteAiModel = (modelId) => {
    const model = aiModels.find((item) => item.id === modelId)
    if (!model || model.builtin) return

    const nextModels = aiModels.filter((item) => item.id !== modelId)
    const updates = { aiModels: nextModels }
    if (settings.aiSelectedModelId === modelId) {
      updates.aiSelectedModelId = DEFAULT_AI_MODEL_ID
    }
    const newSettings = saveSettings(updates)
    onSettingsChange(newSettings)
    message.success('模型配置已删除')
  }

  const handleSelectAiModel = (modelId) => {
    const newSettings = saveSettings({ aiSelectedModelId: modelId })
    onSettingsChange(newSettings)
  }

  return (
    <>
      <div id="ai-settings" className="settings-block">
        <h2>模型设置</h2>
        <div className="settings-section">
          <div className="section-header">
            <h3>内容生成服务</h3>
          </div>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="日报、周报和资料检索会使用当前模型服务"
            description="生成报告时会发送所选任务的工作记录摘要；资料检索还会发送候选归档的路径、标题和归属信息。请仅选择允许发送给当前模型服务的目录。带敏感标识的路径会由后端自动跳过。"
          />

          <div className="ai-current-model">
            <div>
              <div className="ai-current-title">当前配置</div>
              <div className="ai-current-name">{selectedAiModel?.name || '未配置'}</div>
              <div className="ai-current-meta">
                {selectedAiModel?.modelId || '-'} · {selectedAiModel?.apiUrl || '-'}
              </div>
            </div>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => openAiModelModal()}>
              添加模型
            </Button>
          </div>

          <div className="ai-model-list">
            {aiModels.map((model) => (
              <div className={`ai-model-item ${settings.aiSelectedModelId === model.id ? 'active' : ''}`} key={model.id}>
                <div className="ai-model-main">
                  <div className="ai-model-title">
                    <span>{model.name}</span>
                    {model.builtin && <Tag color="blue">内置</Tag>}
                    {settings.aiSelectedModelId === model.id && <Tag color="green">当前</Tag>}
                  </div>
                  <div className="ai-model-meta">{model.modelId}</div>
                  <div className="ai-model-url">{model.apiUrl}</div>
                  <div className="ai-model-key">{model.apiKeyEncrypted ? 'API Key 已保存' : '未保存 API Key'}</div>
                </div>
                <Space>
                  {settings.aiSelectedModelId !== model.id && (
                    <Button onClick={() => handleSelectAiModel(model.id)}>设为当前</Button>
                  )}
                  <Button onClick={() => openAiModelModal(model)}>编辑</Button>
                  {!model.builtin && (
                    <Button danger onClick={() => handleDeleteAiModel(model.id)}>删除</Button>
                  )}
                </Space>
              </div>
            ))}
          </div>
        </div>
      </div>

      <Modal
        title={editingAiModel ? '编辑模型配置' : '添加兼容模型'}
        open={customModelOpen}
        onOk={handleSaveAiModel}
        onCancel={closeAiModelModal}
        okText="保存"
        cancelText="取消"
        destroyOnClose
      >
        <Form form={customModelForm} layout="vertical" preserve={false}>
          <Form.Item
            label="模型名称"
            name="name"
            rules={[{ required: true, message: '请输入模型名称' }]}
          >
            <Input placeholder="例如：DeepSeek V4 Flash" />
          </Form.Item>

          <Form.Item
            label="服务商"
            name="provider"
          >
            <Input placeholder="例如：DeepSeek / OpenAI / 公司内网" />
          </Form.Item>

          <Form.Item
            label="模型 ID"
            name="modelId"
            rules={[
              { required: true, message: '请输入模型 ID' },
              {
                validator: (_, value) => {
                  const model = String(value || '').trim()
                  if (!model) return Promise.resolve()
                  const currentId = editingAiModel?.id
                  const exists = aiModels.some((item) => item.id !== currentId && item.modelId === model)
                  return exists ? Promise.reject(new Error('该模型 ID 已存在')) : Promise.resolve()
                }
              }
            ]}
          >
            <Input placeholder="例如：gpt-4o-mini 或 deepseek-v4-flash" />
          </Form.Item>

          <Form.Item
            label="API 地址"
            name="apiUrl"
            rules={[
              { required: true, message: '请输入 API 地址' },
              { type: 'url', message: '请输入有效的 URL' }
            ]}
          >
            <Input placeholder="例如：https://api.example.com/v1" />
          </Form.Item>

          <Form.Item
            label="API Key"
            tooltip="留空保存时会保留原 API Key"
          >
            <Input.Password
              value={modelApiKey}
              onChange={(e) => setModelApiKey(e.target.value)}
              placeholder="输入后点击保存加密存储"
            />
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}

export default AiSettingsSection
