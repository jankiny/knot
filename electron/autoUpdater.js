const fs = require('fs')
const https = require('https')
const path = require('path')
const semver = require('semver')

const GITHUB_OWNER = 'jankiny'
const GITHUB_REPO = 'knot'
const RELEASES_API = `https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}/releases`
const RELEASES_PAGE = `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/releases`

function createAutoUpdateService({ app, getMainWindow, stopBackend }) {
  let autoUpdater
  let updateInitialized = false
  let updateState = {
    status: 'idle',
    currentVersion: app.getVersion(),
    latestVersion: null,
    availableVersion: null,
    channel: 'stable',
    channelLabel: '正式版',
    previewUpdates: false,
    progress: null,
    releaseTag: null,
    releaseUrl: RELEASES_PAGE,
    manualDownloadUrl: null,
    diagnostics: {},
    message: ''
  }

  function getUpdateChannelInfo() {
    const previewUpdates = isPreviewUpdatesEnabled()
    return {
      channel: previewUpdates ? 'preview' : 'stable',
      channelLabel: previewUpdates ? '预览版' : '正式版',
      previewUpdates
    }
  }

  function sendUpdateState(patch) {
    const channelInfo = getUpdateChannelInfo()
    updateState = {
      ...updateState,
      ...channelInfo,
      ...patch,
      currentVersion: app.getVersion(),
      availableVersion: patch.availableVersion !== undefined ? patch.availableVersion : patch.latestVersion || updateState.availableVersion
    }

    const mainWindow = getMainWindow()
    if (mainWindow && !mainWindow.isDestroyed()) {
      mainWindow.webContents.send('update-status', updateState)
    }

    return updateState
  }

  function serializeUpdateError(error) {
    if (!error) return '更新检查未完成，已保留手动更新入口。'
    if (typeof error === 'string') return error
    const message = error.message || '检查更新失败'
    if (/latest\.ya?ml/i.test(message) || /404/i.test(message)) {
      return '当前发布包缺少自动更新信息，可前往发布页手动下载安装。'
    }
    if (/net::|ENOTFOUND|ECONNRESET|ETIMEDOUT|timeout/i.test(message)) {
      return '无法连接到更新服务，请检查网络后重试。'
    }
    return '更新检查遇到未预期问题，已降级为手动更新方式。'
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

  function requestJson(url) {
    return new Promise((resolve, reject) => {
      const req = https.get(url, {
        headers: {
          Accept: 'application/vnd.github+json',
          'User-Agent': `Knot/${app.getVersion()}`
        },
        timeout: 15000
      }, (res) => {
        let body = ''
        res.setEncoding('utf8')
        res.on('data', (chunk) => {
          body += chunk
        })
        res.on('end', () => {
          if (res.statusCode < 200 || res.statusCode >= 300) {
            const error = new Error(`GitHub release api error: ${res.statusCode}`)
            error.statusCode = res.statusCode
            reject(error)
            return
          }
          try {
            resolve(JSON.parse(body))
          } catch (error) {
            reject(error)
          }
        })
      })

      req.on('timeout', () => {
        req.destroy(new Error('GitHub release api timeout'))
      })
      req.on('error', reject)
    })
  }

  function normalizeVersion(version) {
    return semver.valid(String(version || '').replace(/^v/i, '')) || null
  }

  function getPlatformUpdateAssetNames(release) {
    const assets = Array.isArray(release?.assets) ? release.assets : []
    const names = assets.map((asset) => asset.name)

    if (process.platform === 'win32') {
      return {
        metadata: 'latest.yml',
        packageName: names.find((name) => /^knot-win-.+-x64\.exe$/i.test(name)),
        blockmap: names.find((name) => /^knot-win-.+-x64\.exe\.blockmap$/i.test(name))
      }
    }

    if (process.platform === 'linux') {
      return {
        metadata: 'latest-linux.yml',
        packageName: names.find((name) => /^knot-linux-.+-x86_64\.AppImage$/i.test(name)),
        blockmap: names.find((name) => /^knot-linux-.+-x86_64\.AppImage\.blockmap$/i.test(name))
      }
    }

    return {
      metadata: null,
      packageName: null,
      blockmap: null
    }
  }

  function findAsset(release, assetName) {
    if (!assetName) return null
    return (release.assets || []).find((asset) => asset.name === assetName) || null
  }

  function getReleasePageUrl(release) {
    return release?.html_url || RELEASES_PAGE
  }

  function getManualDownloadUrl(release, packageName) {
    const asset = findAsset(release, packageName)
    return asset?.browser_download_url || getReleasePageUrl(release)
  }

  function releaseDiagnostics(release, assetCheck = {}) {
    const assets = Array.isArray(release?.assets) ? release.assets : []
    const names = assets.map((asset) => asset.name)
    const { metadata, packageName, blockmap } = getPlatformUpdateAssetNames(release)
    return {
      platform: process.platform,
      channel: getUpdateChannelInfo().channel,
      currentVersion: app.getVersion(),
      latestVersion: normalizeVersion(release?.tag_name || release?.name) || null,
      releaseTag: release?.tag_name || null,
      releaseUrl: getReleasePageUrl(release),
      hasMetadata: metadata ? names.includes(metadata) : false,
      hasPackage: packageName ? names.includes(packageName) : false,
      hasBlockmap: blockmap ? names.includes(blockmap) : false,
      metadata,
      packageName,
      blockmap,
      reason: assetCheck.reason || '',
      assets: names
    }
  }

  function validateReleaseAssets(release) {
    const { metadata, packageName, blockmap } = getPlatformUpdateAssetNames(release)
    if (!metadata) {
      return {
        ok: false,
        reason: '当前系统暂不支持自动更新，请前往发布页手动下载安装。',
        packageName
      }
    }

    const metadataAsset = findAsset(release, metadata)
    if (!metadataAsset) {
      return {
        ok: false,
        reason: `当前发布包缺少 ${metadata}，无法自动更新。`,
        packageName
      }
    }
    if (!packageName || !findAsset(release, packageName)) {
      return {
        ok: false,
        reason: '当前发布包缺少对应系统的安装包，无法自动更新。',
        packageName
      }
    }
    if (process.platform === 'win32' && (!blockmap || !findAsset(release, blockmap))) {
      return {
        ok: false,
        reason: '当前发布包缺少增量更新文件，建议手动下载安装。',
        packageName
      }
    }
    return {
      ok: true,
      packageName
    }
  }

  async function fetchTargetRelease(previewUpdates) {
    if (!previewUpdates) {
      return requestJson(`${RELEASES_API}/latest`)
    }

    const releases = await requestJson(`${RELEASES_API}?per_page=30`)
    return (Array.isArray(releases) ? releases : []).find((release) => release.prerelease && !release.draft) || null
  }

  function handledFailureState(error, previewUpdates) {
    const channelLabel = previewUpdates ? '预览版' : '正式版'
    const message = serializeUpdateError(error)
    const isNetwork = /timeout|ENOTFOUND|ECONNRESET|ETIMEDOUT|network|GitHub release api error: (403|429|5\d\d)/i.test(error?.message || '')
    const status = isNetwork ? 'network-unavailable' : 'error-handled'
    return {
      action: 'handled',
      state: {
        status,
        latestVersion: null,
        availableVersion: null,
        releaseTag: null,
        progress: null,
        releaseUrl: RELEASES_PAGE,
        manualDownloadUrl: RELEASES_PAGE,
        previewUpdates,
        diagnostics: {
          platform: process.platform,
          channel: previewUpdates ? 'preview' : 'stable',
          currentVersion: app.getVersion(),
          latestVersion: null,
          releaseTag: null,
          errorType: error?.name || 'Error',
          errorMessage: error?.message || String(error || ''),
          statusCode: error?.statusCode || null
        },
        message: `${channelLabel}更新检查未完成。${message}`
      }
    }
  }

  async function precheckReleaseForUpdates() {
    const previewUpdates = isPreviewUpdatesEnabled()
    const channelLabel = previewUpdates ? '预览版' : '正式版'
    let release
    try {
      release = await fetchTargetRelease(previewUpdates)
    } catch (error) {
      console.error('读取 GitHub Release 失败:', error)
      return handledFailureState(error, previewUpdates)
    }

    if (!release) {
      return {
        action: 'not-available',
        state: {
          status: 'not-available',
          latestVersion: null,
          availableVersion: null,
          releaseTag: null,
          progress: null,
          releaseUrl: RELEASES_PAGE,
          manualDownloadUrl: null,
          previewUpdates,
          diagnostics: {
            platform: process.platform,
            channel: previewUpdates ? 'preview' : 'stable',
            currentVersion: app.getVersion(),
            latestVersion: null,
            releaseTag: null
          },
          message: `暂无可用${channelLabel}更新`
        }
      }
    }

    const currentVersion = normalizeVersion(app.getVersion())
    const latestVersion = normalizeVersion(release.tag_name || release.name)
    const releaseUrl = getReleasePageUrl(release)

    if (currentVersion && latestVersion && !semver.gt(latestVersion, currentVersion)) {
      return {
        action: 'not-available',
        state: {
          status: 'not-available',
          latestVersion,
          availableVersion: latestVersion,
          releaseTag: release.tag_name || null,
          progress: null,
          releaseUrl,
          manualDownloadUrl: null,
          previewUpdates,
          diagnostics: releaseDiagnostics(release),
          message: `当前已是最新${channelLabel}`
        }
      }
    }

    const assetCheck = validateReleaseAssets(release)
    if (!assetCheck.ok) {
      return {
        action: 'incomplete-release',
        state: {
          status: 'incomplete-release',
          latestVersion: latestVersion || release.tag_name || null,
          availableVersion: latestVersion || release.tag_name || null,
          releaseTag: release.tag_name || null,
          progress: null,
          releaseUrl,
          manualDownloadUrl: getManualDownloadUrl(release, assetCheck.packageName),
          previewUpdates,
          diagnostics: releaseDiagnostics(release, assetCheck),
          message: `${channelLabel}发布包不完整：${assetCheck.reason} 可手动下载安装。`
        }
      }
    }

    return {
      action: 'auto',
      state: {
        latestVersion: latestVersion || release.tag_name || null,
        availableVersion: latestVersion || release.tag_name || null,
        releaseTag: release.tag_name || null,
        releaseUrl,
        manualDownloadUrl: getManualDownloadUrl(release, assetCheck.packageName),
        previewUpdates,
        diagnostics: releaseDiagnostics(release, assetCheck),
        message: `发现新的${channelLabel}版本，正在准备下载...`
      }
    }
  }

  function getAutoUpdater() {
    if (autoUpdater) return autoUpdater

    try {
      autoUpdater = require('electron-updater').autoUpdater
      return autoUpdater
    } catch (error) {
      console.error('加载自动更新模块失败:', error)
      sendUpdateState({
        status: 'manual',
        progress: null,
        releaseUrl: RELEASES_PAGE,
        manualDownloadUrl: RELEASES_PAGE,
        diagnostics: {
          platform: process.platform,
          channel: getUpdateChannelInfo().channel,
          currentVersion: app.getVersion(),
          errorMessage: error?.message || String(error)
        },
        message: '自动更新模块不可用，可前往发布页手动下载安装。'
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
        latestVersion: updateState.latestVersion,
        message: '正在检查更新...'
      })
    })

    updater.on('update-available', (info) => {
      sendUpdateState({
        status: 'available',
        latestVersion: info?.version || updateState.latestVersion,
        availableVersion: info?.version || updateState.availableVersion,
        progress: null,
        message: '发现新版本，正在下载...'
      })
    })

    updater.on('update-not-available', () => {
      sendUpdateState({
        status: 'not-available',
        latestVersion: updateState.latestVersion,
        availableVersion: updateState.availableVersion,
        progress: null,
        message: `当前已是最新${getUpdateChannelInfo().channelLabel}`
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
        latestVersion: info?.version || updateState.latestVersion,
        availableVersion: info?.version || updateState.availableVersion,
        progress: { percent: 100 },
        message: '更新已下载，重启应用后安装'
      })
    })

    updater.on('error', (error) => {
      console.error('自动更新失败:', error)
      sendUpdateState({
        status: 'error-handled',
        progress: null,
        manualDownloadUrl: updateState.manualDownloadUrl || updateState.releaseUrl || RELEASES_PAGE,
        diagnostics: {
          ...updateState.diagnostics,
          errorMessage: error?.message || String(error)
        },
        message: serializeUpdateError(error)
      })
    })
  }

  async function checkForUpdates() {
    if (!isAutoUpdateSupported()) {
      return sendUpdateState({
        status: 'unsupported',
        latestVersion: null,
        availableVersion: null,
        progress: null,
        releaseUrl: RELEASES_PAGE,
        manualDownloadUrl: RELEASES_PAGE,
        message: '开发环境不支持自动更新，请使用打包后的应用检查更新'
      })
    }

    try {
      const channelInfo = getUpdateChannelInfo()
      sendUpdateState({
        status: 'checking',
        progress: null,
        latestVersion: null,
        availableVersion: null,
        releaseTag: null,
        manualDownloadUrl: null,
        diagnostics: {
          platform: process.platform,
          channel: channelInfo.channel,
          currentVersion: app.getVersion()
        },
        message: `正在检查${channelInfo.channelLabel}更新...`
      })
      const precheck = await precheckReleaseForUpdates()
      sendUpdateState(precheck.state)
      if (precheck.action !== 'auto') {
        return updateState
      }

      setupAutoUpdater()
      const updater = getAutoUpdater()
      if (!updater) return updateState

      await updater.checkForUpdates()
      return updateState
    } catch (error) {
      console.error('检查更新失败:', error)
      const fallback = handledFailureState(error, isPreviewUpdatesEnabled())
      return sendUpdateState(fallback.state)
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
        status: 'error-handled',
        progress: null,
        manualDownloadUrl: updateState.manualDownloadUrl || updateState.releaseUrl || RELEASES_PAGE,
        diagnostics: {
          ...updateState.diagnostics,
          errorMessage: error?.message || String(error)
        },
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
    getStatus: () => ({
      ...updateState,
      ...getUpdateChannelInfo(),
      currentVersion: app.getVersion()
    }),
    installUpdate,
    isAutoUpdateSupported
  }
}

module.exports = {
  createAutoUpdateService
}
