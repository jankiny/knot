import { message } from 'antd'
import { getSelectedAiModel, getSettings } from '../services/settings'

export async function getReportAiConfig({ enabledSettingKey, featureLabel }) {
  const settings = getSettings()
  const selectedModel = getSelectedAiModel()
  const aiConfig = {
    enabled: !!settings[enabledSettingKey],
    api_url: selectedModel?.apiUrl || '',
    api_key: '',
    model: selectedModel?.modelId || ''
  }

  if (!aiConfig.enabled) {
    return aiConfig
  }

  if (!selectedModel || !aiConfig.api_url || !aiConfig.model || !selectedModel.apiKeyEncrypted) {
    message.warning(`${featureLabel}已启用，但当前模型的 API 地址、模型 ID 或 API Key 未完整配置`)
    return null
  }

  if (window.electronAPI?.decryptPassword) {
    aiConfig.api_key = await window.electronAPI.decryptPassword(selectedModel.apiKeyEncrypted) || ''
  }

  if (!aiConfig.api_key) {
    message.warning('AI Key 解密失败，请重新保存 AI Key')
    return null
  }

  return aiConfig
}
