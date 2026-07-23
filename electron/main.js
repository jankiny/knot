const { app, BrowserWindow, shell, Menu } = require('electron')
const fs = require('fs')
const path = require('path')
const { createAutoUpdateService } = require('./autoUpdater')
const { createBackendProcessManager } = require('./backendProcess')
const { registerIpcHandlers } = require('./ipcHandlers')

// 禁用开发时的 CSP 警告
if (process.env.NODE_ENV === 'development' || !app.isPackaged) {
  process.env['ELECTRON_DISABLE_SECURITY_WARNINGS'] = 'true'
}

let mainWindow

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
    title: 'Knot'
  })

  // 开发模式加载 Vite 开发服务器
  if (process.env.NODE_ENV === 'development' || !app.isPackaged) {
    mainWindow.loadURL('http://localhost:5173')
    mainWindow.webContents.openDevTools()

    // 加载 React DevTools
    try {
      const { default: installExtension, REACT_DEVELOPER_TOOLS } = require('electron-devtools-installer')
      installExtension(REACT_DEVELOPER_TOOLS)
        .then((name) => console.log(`Installed DevTools: ${name}`))
        .catch((err) => console.log('DevTools Installation Error:', err))
    } catch (e) {
      console.log('electron-devtools-installer 未安装，跳过加载 React DevTools')
    }
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

  mainWindow.on('maximize', () => {
    mainWindow.webContents.send('window-maximized-state', true)
  })
  mainWindow.on('unmaximize', () => {
    mainWindow.webContents.send('window-maximized-state', false)
  })
}

const backendManager = createBackendProcessManager({ app, appDir: __dirname })

const updateService = createAutoUpdateService({
  app,
  getMainWindow: () => mainWindow,
  stopBackend: backendManager.stop
})

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
    registerIpcHandlers({
      app,
      getMainWindow: () => mainWindow,
      updateService
    })

    backendManager.start()

    setTimeout(() => {
      createWindow()

      if (updateService.isAutoUpdateSupported()) {
        setTimeout(() => {
          updateService.checkForUpdates()
        }, 15000)
      }
    }, 1000)

    app.on('activate', () => {
      if (BrowserWindow.getAllWindows().length === 0) {
        createWindow()
      }
    })
  })
}

app.on('window-all-closed', () => {
  backendManager.stop()
  if (process.platform !== 'darwin') {
    app.quit()
  }
})

app.on('before-quit', () => {
  backendManager.stop()
})
