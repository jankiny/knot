// 设置管理模块 - 使用 localStorage 持久化

const SETTINGS_KEY = 'knot_settings'

const DEFAULT_SETTINGS = {
  // 窗口样式: 'integrated' (一体化) | 'classic' (经典)
  windowStyle: 'integrated',
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
  // 邮件获取设置
  mailLimit: 50,  // 获取邮件数量限制
  mailDays: 7,    // 获取最近多少天的邮件（0表示不限制）
  // 部门列表
  // { id: 'uuid', name: '部门名称', archivePath: '归档路径' }
  departments: [],
  // 默认部门ID
  defaultDepartmentId: null,
  // 归档扫描目录（扫描工作文件夹的位置）
  scanPath: '~/Desktop'
}

export function getSettings() {
  try {
    const saved = localStorage.getItem(SETTINGS_KEY)
    if (saved) {
      return { ...DEFAULT_SETTINGS, ...JSON.parse(saved) }
    }
  } catch (e) {
    console.error('读取设置失败:', e)
  }
  return DEFAULT_SETTINGS
}

export function saveSettings(updates) {
  try {
    const current = getSettings()
    const newSettings = { ...current, ...updates }
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

export function addDepartment(name, archivePath) {
  const departments = getDepartments()
  const newDept = {
    id: generateId(),
    name,
    archivePath
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
