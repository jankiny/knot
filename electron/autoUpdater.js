const fs = require('fs')
const path = require('path')

function createAutoUpdateService({ app, getMainWindow, stopBackend }) {
  let autoUpdater
  let updateInitialized = false
  let updateState = {
    status: 'idle',
    currentVersion: app.getVersion(),
    availableVersion: null,
    previewUpdates: false,
    progress: null,
    message: ''
  }

  function sendUpdateState(patch) {
    updateState = {
      ...updateState,
      ...patch,
      currentVersion: app.getVersion()
    }

    const mainWindow = getMainWindow()
    if (mainWindow && !mainWindow.isDestroyed()) {
      mainWindow.webContents.send('update-status', updateState)
    }

    return updateState
  }

  function serializeUpdateError(error) {
    if (!error) return '检查更新失败'
    if (typeof error === 'string') return error
    return error.message || '检查更新失败'
  }

  function isAutoUpdateSupported() {
    return app.isPackaged
  }

  function readUserSettings() {
    const settingsPath = path.join(app.getPath('userData'), 'knot-settings.json')
    try {
      if (!fs.existsSync(settingsPath)) return {}
      return JSON.parse(fs.readFileSync(settingsPath, 'utf8'))
    } catch (error) {
      console.error('读取用户设置失败:', error)
      return {}
    }
  }

  function isPreviewUpdatesEnabled() {
    return readUserSettings().enablePreviewUpdates === true
  }

  function getAutoUpdater() {
    if (autoUpdater) return autoUpdater

    try {
      autoUpdater = require('electron-updater').autoUpdater
      return autoUpdater
    } catch (error) {
      console.error('加载自动更新模块失败:', error)
      sendUpdateState({
        status: 'error',
        progress: null,
        message: `自动更新模块加载失败：${serializeUpdateError(error)}`
      })
      return null
    }
  }

  function setupAutoUpdater() {
    const updater = getAutoUpdater()
    if (!updater) return

    updater.allowPrerelease = isPreviewUpdatesEnabled()
    sendUpdateState({ previewUpdates: updater.allowPrerelease })

    if (updateInitialized) return
    updateInitialized = true

    updater.autoDownload = true
    updater.autoInstallOnAppQuit = true

    updater.on('checking-for-update', () => {
      sendUpdateState({
        status: 'checking',
        progress: null,
        message: '正在检查更新...'
      })
    })

    updater.on('update-available', (info) => {
      sendUpdateState({
        status: 'available',
        availableVersion: info?.version || null,
        progress: null,
        message: '发现新版本，正在下载...'
      })
    })

    updater.on('update-not-available', () => {
      sendUpdateState({
        status: 'not-available',
        availableVersion: null,
        progress: null,
        message: '当前已是最新版本'
      })
    })

    updater.on('download-progress', (progress) => {
      sendUpdateState({
        status: 'downloading',
        progress: {
          percent: Math.max(0, Math.min(100, progress?.percent || 0)),
          transferred: progress?.transferred || 0,
          total: progress?.total || 0
        },
        message: '正在下载更新...'
      })
    })

    updater.on('update-downloaded', (info) => {
      sendUpdateState({
        status: 'downloaded',
        availableVersion: info?.version || updateState.availableVersion,
        progress: { percent: 100 },
        message: '更新已下载，重启应用后安装'
      })
    })

    updater.on('error', (error) => {
      sendUpdateState({
        status: 'error',
        progress: null,
        message: serializeUpdateError(error)
      })
    })
  }

  async function checkForUpdates() {
    if (!isAutoUpdateSupported()) {
      return sendUpdateState({
        status: 'unsupported',
        progress: null,
        message: '开发环境不支持自动更新，请使用打包后的应用检查更新'
      })
    }

    setupAutoUpdater()
    const updater = getAutoUpdater()
    if (!updater) return updateState

    try {
      await updater.checkForUpdates()
      return updateState
    } catch (error) {
      return sendUpdateState({
        status: 'error',
        progress: null,
        message: serializeUpdateError(error)
      })
    }
  }

  async function downloadUpdate() {
    if (!isAutoUpdateSupported()) {
      return sendUpdateState({
        status: 'unsupported',
        progress: null,
        message: '开发环境不支持自动更新，请使用打包后的应用检查更新'
      })
    }

    setupAutoUpdater()
    const updater = getAutoUpdater()
    if (!updater) return updateState

    try {
      await updater.downloadUpdate()
      return updateState
    } catch (error) {
      return sendUpdateState({
        status: 'error',
        progress: null,
        message: serializeUpdateError(error)
      })
    }
  }

  function installUpdate() {
    if (updateState.status !== 'downloaded') {
      return {
        ok: false,
        message: '更新尚未下载完成'
      }
    }

    stopBackend()
    setImmediate(() => {
      const updater = getAutoUpdater()
      if (updater) {
        updater.quitAndInstall(false, true)
      }
    })

    return {
      ok: true
    }
  }

  return {
    checkForUpdates,
    downloadUpdate,
    getStatus: () => updateState,
    installUpdate,
    isAutoUpdateSupported
  }
}

module.exports = {
  createAutoUpdateService
}
