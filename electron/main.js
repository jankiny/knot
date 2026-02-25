const { app, BrowserWindow, shell, Menu, ipcMain, dialog, safeStorage } = require('electron')
const fs = require('fs')
const path = require('path')
const { spawn } = require('child_process')

let mainWindow
let backendProcess

function createWindow() {
  // 隐藏菜单栏
  Menu.setApplicationMenu(null)

  // 读取用户设置，判断窗口样式
  const settingsPath = path.join(app.getPath('userData'), 'knot-settings.json')
  let windowStyle = 'integrated'
  try {
    if (fs.existsSync(settingsPath)) {
      const settingsData = fs.readFileSync(settingsPath, 'utf8')
      const parsed = JSON.parse(settingsData)
      if (parsed.windowStyle) {
        windowStyle = parsed.windowStyle
      }
    }
  } catch (err) {
    console.error('读取窗口设置失败:', err)
  }

  const isIntegrated = windowStyle === 'integrated'
  const isDev = process.env.NODE_ENV === 'development' || !app.isPackaged

  mainWindow = new BrowserWindow({
    width: isDev ? 1200 : 900,
    height: 600,
    minWidth: 900,
    minHeight: 600,
    frame: !isIntegrated, // 一体化时隐藏边框，经典时显示边框
    titleBarStyle: isIntegrated ? 'hidden' : 'default', // 在macOS上的表现
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      preload: path.join(__dirname, 'preload.js')
    },
    icon: path.join(__dirname, 'assets/icons/512x512.png'),
    title: 'Knot 绳结'
  })

  // 开发模式加载 Vite 开发服务器
  if (process.env.NODE_ENV === 'development' || !app.isPackaged) {
    mainWindow.loadURL('http://localhost:5173')
    mainWindow.webContents.openDevTools()
  } else {
    // 生产模式加载打包后的文件
    mainWindow.loadFile(path.join(__dirname, '../dist/index.html'))
  }

  // 外部链接在默认浏览器打开
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url)
    return { action: 'deny' }
  })

  mainWindow.on('closed', () => {
    mainWindow = null
  })
}

function startBackend() {
  const fs = require('fs')
  let backendPath

  if (!app.isPackaged) {
    // 开发环境：使用 backend 目录下编译好的可执行文件
    const exeName = process.platform === 'win32' ? 'knot-backend.exe' : 'knot-backend'
    backendPath = path.join(__dirname, '../backend', exeName)

    // 如果开发环境下可执行文件不存在，提示需要先编译
    if (!fs.existsSync(backendPath)) {
      console.error('错误: 后端可执行文件不存在:', backendPath)
      console.error('请先在 backend 目录下运行: go build -o ' + exeName)
      return
    }

    console.log('开发模式 - 正在启动后端:', backendPath)
  } else {
    // 生产环境：使用打包后的可执行文件
    const resourcesPath = process.resourcesPath
    const exeName = process.platform === 'win32' ? 'knot-backend.exe' : 'knot-backend'
    backendPath = path.join(resourcesPath, 'bin', exeName)

    console.log('生产模式 - 资源目录:', resourcesPath)
    console.log('生产模式 - 后端路径:', backendPath)
    console.log('生产模式 - 文件是否存在:', fs.existsSync(backendPath))

    if (!fs.existsSync(backendPath)) {
      console.error('错误: 后端可执行文件不存在:', backendPath)
      return
    }

    // Linux/macOS: 确保后端有执行权限
    if (process.platform !== 'win32') {
      try {
        const stats = fs.statSync(backendPath)
        const mode = stats.mode
        if ((mode & 0o111) === 0) {
          console.log('修复后端执行权限...')
          fs.chmodSync(backendPath, 0o755)
        }
      } catch (err) {
        console.error('检查/设置权限失败:', err)
      }
    }
  }

  backendProcess = spawn(backendPath, [], {
    stdio: ['pipe', 'pipe', 'pipe']
  })

  backendProcess.stdout.on('data', (data) => {
    console.log('后端输出:', data.toString())
  })

  backendProcess.stderr.on('data', (data) => {
    console.error('后端错误/日志:', data.toString())
  })

  backendProcess.on('error', (err) => {
    console.error('启动后端失败:', err)
  })

  backendProcess.on('exit', (code) => {
    console.log(`后端退出，退出码: ${code}`)
  })
}

function stopBackend() {
  if (backendProcess) {
    backendProcess.kill()
    backendProcess = null
  }
}

// 单实例锁
const gotTheLock = app.requestSingleInstanceLock()

if (!gotTheLock) {
  app.quit()
} else {
  app.on('second-instance', (event, commandLine, workingDirectory) => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore()
      mainWindow.focus()
    }
  })

  app.whenReady().then(() => {
    ipcMain.handle('select-folder', async () => {
      const result = await dialog.showOpenDialog(mainWindow, {
        properties: ['openDirectory'],
        title: '选择文件夹'
      })
      if (!result.canceled && result.filePaths.length > 0) {
        return result.filePaths[0]
      }
      return null
    })

    ipcMain.handle('encrypt-password', async (event, password) => {
      if (!password) return null
      try {
        if (safeStorage.isEncryptionAvailable()) {
          const encrypted = safeStorage.encryptString(password)
          return encrypted.toString('base64')
        } else {
          console.warn('safeStorage 加密不可用，使用 base64 编码')
          return Buffer.from(password).toString('base64')
        }
      } catch (err) {
        console.error('加密失败:', err)
        return null
      }
    })

    ipcMain.handle('decrypt-password', async (event, encryptedBase64) => {
      if (!encryptedBase64) return null
      try {
        if (safeStorage.isEncryptionAvailable()) {
          const encrypted = Buffer.from(encryptedBase64, 'base64')
          return safeStorage.decryptString(encrypted)
        } else {
          return Buffer.from(encryptedBase64, 'base64').toString('utf-8')
        }
      } catch (err) {
        console.error('解密失败:', err)
        return null
      }
    })

    ipcMain.handle('is-encryption-available', async () => {
      return safeStorage.isEncryptionAvailable()
    })

    ipcMain.on('get-app-version', (event) => {
      event.returnValue = app.getVersion()
    })

    // 窗口控制事件
    ipcMain.handle('window-minimize', () => {
      if (mainWindow) mainWindow.minimize()
    })

    ipcMain.handle('window-maximize', () => {
      if (mainWindow) {
        if (mainWindow.isMaximized()) {
          mainWindow.unmaximize()
        } else {
          mainWindow.maximize()
        }
      }
    })

    ipcMain.handle('window-close', () => {
      if (mainWindow) mainWindow.close()
    })

    ipcMain.handle('window-is-maximized', () => {
      return mainWindow ? mainWindow.isMaximized() : false
    })

    // 监听窗口最大化/还原事件，通知渲染进程
    if (mainWindow) {
      mainWindow.on('maximize', () => {
        mainWindow.webContents.send('window-maximized-state', true)
      })
      mainWindow.on('unmaximize', () => {
        mainWindow.webContents.send('window-maximized-state', false)
      })
    }

    // 设置项的 IPC 处理（简单存储到 userData 下）
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
      } catch (err) {
        console.error('保存设置失败:', err)
        return false
      }
    })

    // 重启应用
    ipcMain.handle('restart-app', () => {
      app.relaunch()
      app.quit()
    })

    startBackend()

    setTimeout(() => {
      createWindow()
    }, 1000)

    app.on('activate', () => {
      if (BrowserWindow.getAllWindows().length === 0) {
        createWindow()
      }
    })
  })
}

app.on('window-all-closed', () => {
  stopBackend()
  if (process.platform !== 'darwin') {
    app.quit()
  }
})

app.on('before-quit', () => {
  stopBackend()
})
