const { contextBridge, ipcRenderer } = require('electron')

// 尝试读取 package.json 获取版本号，失败则使用默认值
let appVersion = '1.0.0'
try {
  const packageJson = require('../package.json')
  appVersion = packageJson.version
} catch (e) {
  console.warn('Failed to load package.json version:', e)
}

// 暴露安全的 API 给渲染进程
contextBridge.exposeInMainWorld('electronAPI', {
  // 获取平台信息
  platform: process.platform,

  // 获取版本号
  version: appVersion,

  // 获取桌面路径
  getDesktopPath: () => {
    const os = require('os')
    const path = require('path')
    try {
      return path.join(os.homedir(), 'Desktop')
    } catch (e) {
      return ''
    }
  },

  // 选择文件夹
  selectFolder: () => ipcRenderer.invoke('select-folder'),

  // 加密密码
  encryptPassword: (password) => ipcRenderer.invoke('encrypt-password', password),

  // 解密密码
  decryptPassword: (encryptedBase64) => ipcRenderer.invoke('decrypt-password', encryptedBase64),

  // 检查加密是否可用
  isEncryptionAvailable: () => ipcRenderer.invoke('is-encryption-available')
})
