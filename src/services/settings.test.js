import { beforeEach, describe, expect, it } from 'vitest'
import { DEFAULT_AI_MODEL_ID, formatFolderName, getSelectedAiModel, getSettings } from './settings'

const storage = new Map()

globalThis.localStorage = {
  getItem: (key) => storage.get(key) || null,
  setItem: (key, value) => storage.set(key, String(value)),
  removeItem: (key) => storage.delete(key),
  clear: () => storage.clear()
}

beforeEach(() => {
  localStorage.clear()
})

describe('formatFolderName', () => {
  it('formats folder name for mail creation', () => {
    const name = formatFolderName('{{YYYY}}.{{MM}}.{{DD}}_{{subject}}_{{from}}', {
      subject: '项目进度汇报',
      from: '张三 <zhangsan@company.com>',
      date: '2026-04-22 10:00:00'
    })

    expect(name).toBe('2026.04.22_项目进度汇报_张三')
  })

  it('formats folder name for quick creation using work content as subject', () => {
    const name = formatFolderName('{{YYYY}}.{{MM}}.{{DD}}_{{subject}}', {
      subject: '需求:评审/版本1',
      from: '',
      date: '2026-04-20 09:30:00'
    })

    expect(name).toBe('2026.04.20_需求评审版本1')
  })
})

describe('AI model settings', () => {
  it('uses DeepSeek V4 Flash as the default selected model', () => {
    const model = getSelectedAiModel()

    expect(model.id).toBe(DEFAULT_AI_MODEL_ID)
    expect(model.apiUrl).toBe('https://api.deepseek.com')
    expect(model.modelId).toBe('deepseek-v4-flash')
  })

  it('migrates legacy AI settings into model configuration', () => {
    localStorage.setItem('knot_settings', JSON.stringify({
      aiApiUrl: 'https://api.example.com/v1',
      aiModel: 'custom-model',
      aiApiKeyEncrypted: 'encrypted-key'
    }))

    const settings = getSettings()
    const model = getSelectedAiModel()

    expect(settings.aiSelectedModelId).toBe('custom-model')
    expect(model).toMatchObject({
      id: 'custom-model',
      apiUrl: 'https://api.example.com/v1',
      modelId: 'custom-model',
      apiKeyEncrypted: 'encrypted-key',
      builtin: false
    })
  })
})
