const fs = require('fs')
const path = require('path')
const { spawn } = require('child_process')

const KNOT_DATA_DIR_ENV = 'KNOT_DATA_DIR'

function buildBackendEnvironment(app, baseEnvironment = process.env) {
  return {
    ...baseEnvironment,
    [KNOT_DATA_DIR_ENV]: app.getPath('userData')
  }
}

function createBackendProcessManager({ app, appDir }) {
  let backendProcess = null

  function getBackendPath() {
    const exeName = process.platform === 'win32' ? 'knot-backend.exe' : 'knot-backend'

    if (!app.isPackaged) {
      return {
        backendPath: path.join(appDir, '../backend', exeName),
        exeName,
        mode: 'development'
      }
    }

    return {
      backendPath: path.join(process.resourcesPath, 'bin', exeName),
      exeName,
      mode: 'production'
    }
  }

  function ensureExecutable(filePath) {
    if (process.platform === 'win32') return

    try {
      const stats = fs.statSync(filePath)
      if ((stats.mode & 0o111) === 0) {
        console.log('修复后端执行权限...')
        fs.chmodSync(filePath, 0o755)
      }
    } catch (error) {
      console.error('检查/设置权限失败:', error)
    }
  }

  function start() {
    if (backendProcess) {
      return
    }

    const { backendPath, exeName, mode } = getBackendPath()
    if (!fs.existsSync(backendPath)) {
      console.error('错误: 后端可执行文件不存在:', backendPath)
      if (mode === 'development') {
        console.error('请先在 backend 目录下运行: go build -o ' + exeName)
      }
      return
    }

    if (mode === 'production') {
      console.log('生产模式 - 后端路径:', backendPath)
      ensureExecutable(backendPath)
    } else {
      console.log('开发模式 - 正在启动后端:', backendPath)
    }

    backendProcess = spawn(backendPath, [], {
      env: buildBackendEnvironment(app),
      stdio: ['pipe', 'pipe', 'pipe']
    })

    backendProcess.stdout.on('data', (data) => {
      console.log('后端输出:', data.toString())
    })

    backendProcess.stderr.on('data', (data) => {
      console.error('后端错误/日志:', data.toString())
    })

    backendProcess.on('error', (error) => {
      console.error('启动后端失败:', error)
      backendProcess = null
    })

    backendProcess.on('exit', (code) => {
      console.log(`后端退出，退出码: ${code}`)
      backendProcess = null
    })
  }

  function stop() {
    if (backendProcess) {
      backendProcess.kill()
      backendProcess = null
    }
  }

  return {
    start,
    stop
  }
}

module.exports = {
  KNOT_DATA_DIR_ENV,
  buildBackendEnvironment,
  createBackendProcessManager
}
