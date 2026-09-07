// 设置管理模块 - 使用 localStorage 持久化

import {
  BUILTIN_AI_MODELS,
  DEFAULT_AI_MODEL_ID,
  normalizeAiSettings
} from './aiSettings'

export { BUILTIN_AI_MODELS, DEFAULT_AI_MODEL_ID }

const SETTINGS_KEY = 'knot_settings'

const DEFAULT_SETTINGS = {
  // 窗口样式: 'integrated' (一体化) | 'classic' (经典)
  windowStyle: 'integrated',
  // 应用标题语言: 'en' (Knot) | 'zh' (绳结)
  appTitleLanguage: 'en',
  // 开发者模式下可查看更新诊断信息
  developerMode: false,
  folderPath: '~/Desktop',  // 默认桌面
  // 文件夹命名格式，支持变量：
  // {{YYYY}} - 年份，{{MM}} - 月份，{{DD}} - 日期
  // {{subject}} - 邮件主题，{{from}} - 发件人
  folderNameFormat: '{{YYYY}}.{{MM}}.{{DD}}_{{subject}}',
  // 是否添加子目录
  useSubFolder: false,
  // 子目录名称
  subFolderName: '邮件',
  // 是否保存邮件正文
  saveMailContent: true,
  // 邮件正文文件名（不含扩展名，扩展名由保存格式决定）
  mailContentFileName: '邮件正文',
  // 邮件保存格式：txt, eml, pdf（可多选）
  saveFormats: ['txt'],
  // 邮件服务器配置
  mailServer: '',
  mailPort: 993,
  mailUsername: '',
  mailPasswordEncrypted: null,  // 加密存储的密码
  mailUseSsl: true,
  mailInsecureSkipVerify: false,
  // 邮件获取设置
  mailLimit: 50,  // 获取邮件数量限制
  mailDays: 7,    // 获取最近多少天的邮件（0表示不限制）
  // 部门列表
  // { id: 'uuid', name: '部门名称', archivePath: '归档路径' }
  departments: [],
  // 默认部门ID
  defaultDepartmentId: null,
  projects: [],
  defaultProjectId: null,
  defaultSopTemplateId: 'default-task',
  // 归档扫描目录（扫描工作文件夹的位置）
  scanPath: '~/Desktop',
  // AI 日报设置
  aiApiUrl: 'https://api.deepseek.com',
  aiModel: 'deepseek-v4-flash',
  aiApiKeyEncrypted: null,
  aiCustomModels: [],
  aiSelectedModelId: DEFAULT_AI_MODEL_ID,
  aiModels: BUILTIN_AI_MODELS,
  // 是否跟踪 preview/alpha 预览版更新
  enablePreviewUpdates: false
}

function normalizeSettings(settings) {
  return {
    ...settings,
    appTitleLanguage: settings.appTitleLanguage === 'zh' ? 'zh' : 'en',
    developerMode: settings.developerMode === true,
    mailInsecureSkipVerify: settings.mailUseSsl !== false && settings.mailInsecureSkipVerify === true,
    ...normalizeAiSettings(settings)
  }
}

export function getSettings() {
  try {
    const saved = localStorage.getItem(SETTINGS_KEY)
    if (saved) {
      const parsed = JSON.parse(saved)
      const merged = { ...DEFAULT_SETTINGS, ...parsed }
      if (!Object.prototype.hasOwnProperty.call(parsed, 'aiSelectedModelId') && parsed.aiModel) {
        merged.aiSelectedModelId = parsed.aiModel
      }
      return normalizeSettings(merged)
    }
  } catch (e) {
    console.error('读取设置失败:', e)
  }
  return normalizeSettings(DEFAULT_SETTINGS)
}

export function saveSettings(updates) {
  try {
    const current = getSettings()
    const mailboxChanged = ['mailServer', 'mailPort', 'mailUsername', 'mailUseSsl'].some(
      (key) => Object.prototype.hasOwnProperty.call(updates, key) && updates[key] !== current[key]
    )
    if (mailboxChanged && !Object.prototype.hasOwnProperty.call(updates, 'mailInsecureSkipVerify')) {
      updates = { ...updates, mailInsecureSkipVerify: false }
    }
    const newSettings = normalizeSettings({ ...current, ...updates })
    localStorage.setItem(SETTINGS_KEY, JSON.stringify(newSettings))
    return newSettings
  } catch (e) {
    console.error('保存设置失败:', e)
    return getSettings()
  }
}

export function getFolderPath() {
  return getSettings().folderPath
}

// 生成唯一ID
function generateId() {
  return Date.now().toString(36) + Math.random().toString(36).substr(2)
}

// 部门管理 CRUD 操作
export function getDepartments() {
  return getSettings().departments || []
}

export function addDepartment(name, archivePath, useYearFolder = true) {
  const departments = getDepartments()
  const newDept = {
    id: generateId(),
    name,
    archivePath,
    useYearFolder
  }
  departments.push(newDept)
  saveSettings({ departments })
  return newDept
}

export function updateDepartment(id, updates) {
  const departments = getDepartments()
  const index = departments.findIndex(d => d.id === id)
  if (index !== -1) {
    departments[index] = { ...departments[index], ...updates }
    saveSettings({ departments })
    return departments[index]
  }
  return null
}

export function deleteDepartment(id) {
  const departments = getDepartments()
  const filtered = departments.filter(d => d.id !== id)
  const settings = getSettings()
  // 如果删除的是默认部门，清除默认部门设置
  if (settings.defaultDepartmentId === id) {
    saveSettings({ departments: filtered, defaultDepartmentId: null })
  } else {
    saveSettings({ departments: filtered })
  }
  return filtered
}

export function getDepartmentById(id) {
  const departments = getDepartments()
  return departments.find(d => d.id === id) || null
}

export function setDefaultDepartment(id) {
  saveSettings({ defaultDepartmentId: id })
}

export function getDefaultDepartment() {
  const settings = getSettings()
  if (settings.defaultDepartmentId) {
    return getDepartmentById(settings.defaultDepartmentId)
  }
  return null
}

export function getAiModels() {
  return getSettings().aiModels || BUILTIN_AI_MODELS
}

export function getSelectedAiModel() {
  const settings = getSettings()
  const models = settings.aiModels || BUILTIN_AI_MODELS
  return models.find((model) => model.id === settings.aiSelectedModelId) ||
    models.find((model) => model.id === DEFAULT_AI_MODEL_ID) ||
    models[0] ||
    null
}

export function getProjects() {
  return getSettings().projects || []
}

export function addProject(name, archivePath, useYearFolder = false) {
  const projects = getProjects()
  const newProject = {
    id: generateId(),
    name,
    archivePath,
    useYearFolder
  }
  projects.push(newProject)
  saveSettings({ projects })
  return newProject
}

export function updateProject(id, updates) {
  const projects = getProjects()
  const index = projects.findIndex(p => p.id === id)
  if (index !== -1) {
    projects[index] = { ...projects[index], ...updates }
    saveSettings({ projects })
    return projects[index]
  }
  return null
}

export function deleteProject(id) {
  const projects = getProjects()
  const filtered = projects.filter(p => p.id !== id)
  const settings = getSettings()
  if (settings.defaultProjectId === id) {
    saveSettings({ projects: filtered, defaultProjectId: null })
  } else {
    saveSettings({ projects: filtered })
  }
  return filtered
}

export function getProjectById(id) {
  const projects = getProjects()
  return projects.find(p => p.id === id) || null
}

export function setDefaultProject(id) {
  saveSettings({ defaultProjectId: id })
}

export function getDefaultProject() {
  const settings = getSettings()
  if (settings.defaultProjectId) {
    return getProjectById(settings.defaultProjectId)
  }
  return null
}

export function getDefaultSopTemplateId() {
  return getSettings().defaultSopTemplateId || 'default-task'
}

export function setDefaultSopTemplateId(id) {
  saveSettings({ defaultSopTemplateId: id || 'default-task' })
}

// 清理邮件主题，用于生成文件夹名称
export function cleanSubjectForFolder(subject) {
  if (!subject) return ''

  let cleaned = subject

  // 删除【】符号及其内容
  cleaned = cleaned.replace(/【[^】]*】/g, '')

  // 删除转发/回复前缀（支持多种语言）
  const prefixes = [
    /^转发[：:]\s*/i,
    /^转寄[：:]\s*/i,
    /^回复[：:]\s*/i,
    /^答复[：:]\s*/i,
    /^Fwd?[：:]\s*/i,
    /^Re[：:]\s*/i,
    /^Fw[：:]\s*/i,
  ]

  // 可能有多个前缀，循环删除
  let prevLength
  do {
    prevLength = cleaned.length
    for (const prefix of prefixes) {
      cleaned = cleaned.replace(prefix, '')
    }
  } while (cleaned.length !== prevLength && cleaned.length > 0)

  // 去除首尾空格
  cleaned = cleaned.trim()

  return cleaned
}

// 根据格式生成文件夹名称
export function formatFolderName(format, mail) {
  const date = new Date(mail.date)
  const year = date.getFullYear().toString()
  const month = (date.getMonth() + 1).toString().padStart(2, '0')
  const day = date.getDate().toString().padStart(2, '0')

  // 从发件人中提取名称
  let fromName = mail.from || ''
  const match = fromName.match(/^([^<]+)/)
  if (match) {
    fromName = match[1].trim()
  }

  // 清理主题：删除【】内容、转发前缀等，再删除非法字符
  const cleanedSubject = cleanSubjectForFolder(mail.subject || '')
  const safeSubject = cleanedSubject.replace(/[\\/:*?"<>|]/g, '').slice(0, 50)
  const safeFrom = fromName.replace(/[\\/:*?"<>|]/g, '').slice(0, 20)

  return format
    .replace(/\{\{YYYY\}\}/g, year)
    .replace(/\{\{MM\}\}/g, month)
    .replace(/\{\{DD\}\}/g, day)
    .replace(/\{\{subject\}\}/g, safeSubject)
    .replace(/\{\{from\}\}/g, safeFrom)
}

// 生成标识 hash（SHA-256 前16位 hex）
// 邮件来源：generateHash(subject + '|' + date + '|' + from)
// 快速创建：generateHash(folderName)
export async function generateHash(input) {
  const encoder = new TextEncoder()
  const data = encoder.encode(input)
  const hashBuffer = await crypto.subtle.digest('SHA-256', data)
  const hashArray = Array.from(new Uint8Array(hashBuffer))
  const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join('')
  return hashHex.slice(0, 16)
}

// 为邮件生成标识 hash
export async function generateMailHash(mail) {
  const input = `${mail.subject || ''}|${mail.date || ''}|${mail.from || ''}`
  return generateHash(input)
}

// 为快速创建生成标识 hash
export async function generateFolderHash(folderName) {
  return generateHash(folderName)
}
