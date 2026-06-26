const fs = require('fs')
const path = require('path')
const { dialog, ipcMain, safeStorage, shell } = require('electron')

function registerIpcHandlers({ app, getMainWindow, updateService }) {
  ipcMain.handle('select-folder', async () => {
    const result = await dialog.showOpenDialog(getMainWindow(), {
      properties: ['openDirectory'],
      title: '选择文件夹'
    })
    if (!result.canceled && result.filePaths.length > 0) {
      return result.filePaths[0]
    }
    return null
  })

  ipcMain.handle('open-folder', async (event, folderPath) => {
    if (!folderPath) return false
    try {
      const error = await shell.openPath(folderPath)
      return error === ''
    } catch (error) {
      console.error('打开目录失败:', error)
      return false
    }
  })

  ipcMain.handle('open-external', async (event, url) => {
    if (!url || !/^https?:\/\//i.test(url)) return false
    try {
      await shell.openExternal(url)
      return true
    } catch (error) {
      console.error('打开外部链接失败:', error)
      return false
    }
  })

  ipcMain.handle('encrypt-password', async (event, password) => {
    if (!password) return null
    try {
      if (safeStorage.isEncryptionAvailable()) {
        const encrypted = safeStorage.encryptString(password)
        return encrypted.toString('base64')
      }
      console.warn('safeStorage 加密不可用，使用 base64 编码')
      return Buffer.from(password).toString('base64')
    } catch (error) {
      console.error('加密失败:', error)
      return null
    }
  })

  ipcMain.handle('decrypt-password', async (event, encryptedBase64) => {
    if (!encryptedBase64) return null
    try {
      if (safeStorage.isEncryptionAvailable()) {
        const encrypted = Buffer.from(encryptedBase64, 'base64')
        return safeStorage.decryptString(encrypted)
      }
      return Buffer.from(encryptedBase64, 'base64').toString('utf-8')
    } catch (error) {
      console.error('解密失败:', error)
      return null
    }
  })

  ipcMain.handle('is-encryption-available', async () => {
    return safeStorage.isEncryptionAvailable()
  })

  ipcMain.on('get-app-version', (event) => {
    event.returnValue = app.getVersion()
  })

  ipcMain.handle('update-get-status', () => {
    return updateService.getStatus()
  })

  ipcMain.handle('update-check', async () => {
    return updateService.checkForUpdates()
  })

  ipcMain.handle('update-download', async () => {
    return updateService.downloadUpdate()
  })

  ipcMain.handle('update-install', () => {
    return updateService.installUpdate()
  })

  ipcMain.handle('window-minimize', () => {
    const mainWindow = getMainWindow()
    if (mainWindow) mainWindow.minimize()
  })

  ipcMain.handle('window-maximize', () => {
    const mainWindow = getMainWindow()
    if (!mainWindow) return
    if (mainWindow.isMaximized()) {
      mainWindow.unmaximize()
    } else {
      mainWindow.maximize()
    }
  })

  ipcMain.handle('window-close', () => {
    const mainWindow = getMainWindow()
    if (mainWindow) mainWindow.close()
  })

  ipcMain.handle('window-is-maximized', () => {
    const mainWindow = getMainWindow()
    return mainWindow ? mainWindow.isMaximized() : false
  })

  ipcMain.handle('save-setting', (event, key, value) => {
    try {
      const settingsPath = path.join(app.getPath('userData'), 'knot-settings.json')
      let settings = {}
      if (fs.existsSync(settingsPath)) {
        settings = JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
      }
      settings[key] = value
      fs.writeFileSync(settingsPath, JSON.stringify(settings, null, 2), 'utf8')
      return true
    } catch (error) {
      console.error('保存设置失败:', error)
      return false
    }
  })

  ipcMain.handle('restart-app', () => {
    app.relaunch()
    app.quit()
  })
}

module.exports = {
  registerIpcHandlers
}
