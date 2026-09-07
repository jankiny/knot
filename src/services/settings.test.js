import { beforeEach, describe, expect, it } from 'vitest'
import { DEFAULT_AI_MODEL_ID, formatFolderName, getSelectedAiModel, getSettings, saveSettings } from './settings'
import { readMailCache, saveMailCache } from './mailCache'

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

describe('mail certificate compatibility', () => {
  const mailbox = { mailServer: 'imap.internal.test', mailPort: 993, mailUsername: 'user', mailUseSsl: true }

  it('keeps certificate verification enabled for new and legacy settings', () => {
    expect(getSettings().mailInsecureSkipVerify).toBe(false)
    localStorage.setItem('knot_settings', JSON.stringify(mailbox))
    expect(getSettings().mailInsecureSkipVerify).toBe(false)
    localStorage.setItem('knot_settings', JSON.stringify({ ...mailbox, mailInsecureSkipVerify: 'true' }))
    expect(getSettings().mailInsecureSkipVerify).toBe(false)
  })

  it('persists an explicit exception through reloads and unrelated settings changes', () => {
    saveSettings({ ...mailbox, mailInsecureSkipVerify: true })
    saveSettings({ mailDays: 30 })
    expect(getSettings().mailInsecureSkipVerify).toBe(true)
  })

  it.each([
    { mailServer: 'imap.public.test' },
    { mailPort: 994 },
    { mailUsername: 'another-user' },
    { mailUseSsl: false }
  ])('resets an existing exception when the mailbox changes: %j', (updates) => {
    saveSettings({ ...mailbox, mailInsecureSkipVerify: true })
    saveSettings(updates)
    expect(getSettings().mailInsecureSkipVerify).toBe(false)
  })

  it('requires TLS even when an explicit exception is supplied', () => {
    saveSettings({ ...mailbox, mailUseSsl: false, mailInsecureSkipVerify: true })
    expect(getSettings().mailInsecureSkipVerify).toBe(false)
  })

  it('does not reuse mail cached with a different certificate policy', () => {
    const settings = { ...mailbox, mailInsecureSkipVerify: true }
    saveMailCache(settings, [{ id: '1', subject: 'cached mail' }])
    expect(readMailCache(settings)?.mails).toHaveLength(1)
    expect(readMailCache({ ...settings, mailInsecureSkipVerify: false })).toBeNull()
    saveMailCache(mailbox, [{ id: '2' }])
    expect(readMailCache(settings)).toBeNull()
    expect(readMailCache(mailbox)?.mails).toHaveLength(1)
  })
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

describe('appearance settings', () => {
  it('uses the English title and hides developer tools by default', () => {
    expect(getSettings()).toMatchObject({
      appTitleLanguage: 'en',
      developerMode: false
    })
  })

  it('restores the selected title language and developer mode', () => {
    localStorage.setItem('knot_settings', JSON.stringify({
      appTitleLanguage: 'zh',
      developerMode: true
    }))

    expect(getSettings()).toMatchObject({
      appTitleLanguage: 'zh',
      developerMode: true
    })
  })
})
