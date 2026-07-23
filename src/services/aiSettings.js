export const DEFAULT_AI_MODEL_ID = 'deepseek-v4-flash'

export const BUILTIN_AI_MODELS = [
  {
    id: 'deepseek-v4-flash',
    name: 'DeepSeek V4 Flash',
    provider: 'DeepSeek',
    apiUrl: 'https://api.deepseek.com',
    modelId: 'deepseek-v4-flash',
    apiKeyEncrypted: null,
    builtin: true
  },
  {
    id: 'deepseek-v4-pro',
    name: 'DeepSeek V4 Pro',
    provider: 'DeepSeek',
    apiUrl: 'https://api.deepseek.com',
    modelId: 'deepseek-v4-pro',
    apiKeyEncrypted: null,
    builtin: true
  }
]

export function normalizeAiModel(model) {
  if (!model) return null
  const modelId = String(model.modelId || model.model || model.id || '').trim()
  if (!modelId) return null
  const apiUrl = String(model.apiUrl || '').trim()
  return {
    id: String(model.id || modelId).trim(),
    name: String(model.name || model.label || modelId).trim(),
    provider: String(model.provider || (model.builtin ? 'DeepSeek' : 'custom-openai')).trim(),
    apiUrl,
    modelId,
    apiKeyEncrypted: model.apiKeyEncrypted || model.api_key_encrypted || null,
    builtin: !!model.builtin
  }
}

export function mergeAiModels(saved = {}) {
  const merged = new Map()

  BUILTIN_AI_MODELS.forEach((model) => {
    merged.set(model.id, { ...model })
  })

  ;(saved.aiModels || []).forEach((model) => {
    const normalized = normalizeAiModel(model)
    if (!normalized) return
    const existing = merged.get(normalized.id)
    merged.set(normalized.id, {
      ...(existing || {}),
      ...normalized,
      builtin: existing?.builtin || normalized.builtin
    })
  })

  ;(saved.aiCustomModels || []).forEach((model) => {
    const normalized = normalizeAiModel({
      id: model.id || model.model,
      name: model.name || model.model,
      provider: model.provider || 'custom-openai',
      apiUrl: model.apiUrl,
      modelId: model.model,
      apiKeyEncrypted: model.apiKeyEncrypted,
      builtin: false
    })
    if (!normalized || merged.has(normalized.id)) return
    merged.set(normalized.id, normalized)
  })

  const legacyModel = String(saved.aiModel || '').trim()
  const legacyApiUrl = String(saved.aiApiUrl || '').trim()
  if (legacyModel) {
    const legacyId = legacyModel
    const existing = merged.get(legacyId)
    if (existing) {
      merged.set(legacyId, {
        ...existing,
        apiUrl: legacyApiUrl || existing.apiUrl,
        apiKeyEncrypted: saved.aiApiKeyEncrypted || existing.apiKeyEncrypted || null
      })
    } else {
      merged.set(legacyId, {
        id: legacyId,
        name: legacyModel,
        provider: 'custom-openai',
        apiUrl: legacyApiUrl || 'https://api.deepseek.com',
        modelId: legacyModel,
        apiKeyEncrypted: saved.aiApiKeyEncrypted || null,
        builtin: false
      })
    }
  }

  return Array.from(merged.values())
}

export function normalizeAiSettings(settings) {
  const aiModels = mergeAiModels(settings)
  const selected = settings.aiSelectedModelId || settings.aiModel || DEFAULT_AI_MODEL_ID
  const selectedExists = aiModels.some((model) => model.id === selected)
  return {
    aiModels,
    aiSelectedModelId: selectedExists ? selected : DEFAULT_AI_MODEL_ID
  }
}
