import axios from 'axios'
import { mockApi } from './mockData'
import { getSettings, formatFolderName } from './settings'

// 生产环境下直接指向后端端口，开发环境下走 Vite 代理
const API_BASE = import.meta.env.DEV ? '/api' : 'http://localhost:18000/api'

// 是否使用 Mock 模式（外网开发时设为 true）
// Mock 模式只影响邮件获取，文件夹创建仍通过后端实现
export const USE_MOCK = false

export const mailApi = {
  // 连接邮件服务器
  connect: async (config) => {
    if (USE_MOCK) return mockApi.connect(config)
    const response = await axios.post(`${API_BASE}/mail/connect`, config)
    return response.data
  },

  // 获取邮件列表
  getMailList: async (limit = 50, days = 7) => {
    if (USE_MOCK) return mockApi.getMailList()
    const response = await axios.get(`${API_BASE}/mail/list`, { params: { limit, days } })
    return response.data
  },

  // 获取邮件详情（正文和附件信息）
  getMailDetail: async (mailId) => {
    if (USE_MOCK) return mockApi.getMailDetail ? mockApi.getMailDetail(mailId) : { success: true, data: { body: '', attachments: [] } }
    const response = await axios.get(`${API_BASE}/mail/${mailId}/detail`)
    return response.data
  },

  // 获取邮件附件
  getAttachments: async (mailId) => {
    if (USE_MOCK) return mockApi.getAttachments(mailId)
    const response = await axios.get(`${API_BASE}/mail/${mailId}/attachments`)
    return response.data
  }
}

export const folderApi = {
  // 创建文件夹 - 始终调用后端 API 以真正创建文件夹
  create: async (requestData) => {
    try {
      const response = await axios.post(`${API_BASE}/folder/create`, requestData)
      return response.data
    } catch (error) {
      // 如果后端不可用，返回模拟结果（仅用于 UI 测试）
      if (USE_MOCK && error.code === 'ERR_NETWORK') {
        return {
          success: true,
          path: `${requestData.base_path}/${requestData.folder_name}`,
          message: `文件夹已创建: ${requestData.folder_name} (模拟)`
        }
      }
      throw error
    }
  },

  // 创建文件夹并下载附件
  createWithAttachments: async (requestData) => {
    try {
      const response = await axios.post(`${API_BASE}/folder/create-with-attachments`, requestData)
      return response.data
    } catch (error) {
      // 如果后端不可用，返回模拟结果（仅用于 UI 测试）
      if (USE_MOCK && error.code === 'ERR_NETWORK') {
        return {
          success: true,
          path: `${requestData.base_path}/${requestData.folder_name}`,
          attachments_downloaded: requestData.attachments?.map(a => a.filename) || [],
          message: `文件夹已创建 (模拟)`
        }
      }
      throw error
    }
  }
}

export const archiveApi = {
  // 扫描工作文件夹
  scan: async (scanPath) => {
    try {
      const response = await axios.get(`${API_BASE}/archive/scan`, {
        params: { scan_path: scanPath }
      })
      return response.data
    } catch (error) {
      // Mock 模式下返回模拟数据
      if (USE_MOCK && error.code === 'ERR_NETWORK') {
        return {
          success: true,
          scan_path: scanPath,
          count: 3,
          folders: [
            {
              name: '2025.01.15_项目进度汇报',
              path: `${scanPath}/2025.01.15_项目进度汇报`,
              created: '2025-01-15T10:30:00',
              modified: '2025-01-15T14:20:00',
              has_work_record: true,
              department: '科数部',
              create_time: '2025-01-15 10:30'
            },
            {
              name: '2025.01.16_系统维护通知',
              path: `${scanPath}/2025.01.16_系统维护通知`,
              created: '2025-01-16T09:00:00',
              modified: '2025-01-16T11:30:00',
              has_work_record: true,
              department: '技术部',
              create_time: '2025-01-16 09:00'
            },
            {
              name: '2025.01.17_会议纪要',
              path: `${scanPath}/2025.01.17_会议纪要`,
              created: '2025-01-17T15:00:00',
              modified: '2025-01-17T16:45:00',
              has_work_record: false,
              department: null,
              create_time: null
            }
          ]
        }
      }
      throw error
    }
  },

  // 移动单个文件夹到归档目录
  move: async (folderPath, archivePath) => {
    try {
      const response = await axios.post(`${API_BASE}/archive/move`, {
        folder_path: folderPath,
        archive_path: archivePath
      })
      return response.data
    } catch (error) {
      if (USE_MOCK && error.code === 'ERR_NETWORK') {
        const folderName = folderPath.split('/').pop()
        const year = folderName.substring(0, 4)
        return {
          success: true,
          source: folderPath,
          destination: `${archivePath}/${year}/${folderName}`,
          message: `已归档到: ${archivePath}/${year}/${folderName} (模拟)`
        }
      }
      throw error
    }
  },

  // 批量归档
  batchMove: async (items) => {
    try {
      const response = await axios.post(`${API_BASE}/archive/batch-move`, { items })
      return response.data
    } catch (error) {
      if (USE_MOCK && error.code === 'ERR_NETWORK') {
        return {
          success: true,
          total: items.length,
          success_count: items.length,
          fail_count: 0,
          results: items.map(item => ({
            source: item.folder_path,
            destination: `${item.archive_path}/2025/${item.folder_path.split('/').pop()}`,
            success: true,
            message: '归档成功 (模拟)'
          }))
        }
      }
      throw error
    }
  }
}
