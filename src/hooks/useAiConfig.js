import { message } from 'antd'
import { getSelectedAiModel } from '../services/settings'

export async function getReportAiConfig({ featureLabel }) {
  const selectedModel = getSelectedAiModel()
  const aiConfig = {
    enabled: true,
    api_url: selectedModel?.apiUrl || '',
    api_key: '',
    model: selectedModel?.modelId || ''
  }

  if (!selectedModel || !aiConfig.api_url || !aiConfig.model || !selectedModel.apiKeyEncrypted) {
    message.warning(`${featureLabel}需要完整配置当前模型的 API 地址、模型 ID 和 API Key`)
    return null
  }

  if (window.electronAPI?.decryptPassword) {
    aiConfig.api_key = await window.electronAPI.decryptPassword(selectedModel.apiKeyEncrypted) || ''
  }

  if (!aiConfig.api_key) {
    message.warning('API Key 解密失败，请重新保存 API Key')
    return null
  }

  return aiConfig
}
