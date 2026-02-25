const { contextBridge, ipcRenderer } = require('electron')

// 获取版本号（同步方式，在预加载时获取一次即可保证最新）
const appVersion = ipcRenderer.sendSync('get-app-version')

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
