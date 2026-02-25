import { useState } from 'react'
import { Card, Input, Button, DatePicker, message, Typography, Space, Divider } from 'antd'
import { FolderAddOutlined, CalendarOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { folderApi } from '../services/api'
import { getSettings, getDepartments } from '../services/settings'
import DepartmentSelectModal from './DepartmentSelectModal'
import './QuickCreate.css'

const { Title, Text } = Typography
const { TextArea } = Input

function QuickCreate() {
  const [workContent, setWorkContent] = useState('')
  const [selectedDate, setSelectedDate] = useState(dayjs())
  const [creating, setCreating] = useState(false)
  const [deptModalOpen, setDeptModalOpen] = useState(false)

  // 生成文件夹名称预览
  const getFolderNamePreview = () => {
    if (!workContent.trim()) return ''
    const dateStr = selectedDate.format('YYYY.MM.DD')
    const safeContent = workContent.trim().replace(/[\\/:*?"<>|]/g, '').slice(0, 50)
    return `${dateStr}_${safeContent}`
  }

  // 打开部门选择弹窗
  const handleCreate = () => {
    if (!workContent.trim()) {
      message.warning('请输入工作内容')
      return
    }
    setDeptModalOpen(true)
  }

  // 确认选择部门后创建文件夹
  const handleDeptConfirm = async (department) => {
    setDeptModalOpen(false)
    await createFolder(department)
  }

  // 创建文件夹
  const createFolder = async (department = null) => {
    setCreating(true)
    try {
      const settings = getSettings()
      const folderName = getFolderNamePreview()

      const requestData = {
        base_path: settings.folderPath,
        folder_name: folderName,
        // 手动创建，无邮件相关内容
        mail_id: null,
        subject: workContent.trim(),
        date: selectedDate.toISOString(),
        from_addr: '',
        body: '',
        use_sub_folder: false,
        save_mail_content: false,
        attachments: [],
        // 部门信息
        department: department ? department.name : null,
        source: '快速创建'
      }

      const result = await folderApi.create(requestData)
      message.success(result.message || `文件夹已创建: ${folderName}`)

      // 清空输入
      setWorkContent('')
      setSelectedDate(dayjs())
    } catch (error) {
      message.error(error.response?.data?.detail || '创建文件夹失败')
    } finally {
      setCreating(false)
    }
  }

  const folderPreview = getFolderNamePreview()

  return (
    <div className="quick-create">
      <Card variant="borderless">
        <Title level={4}>快速创建工作文件夹</Title>
        <Text type="secondary">
          用于非邮件来源的工作任务，创建后可使用自动归档功能
        </Text>

        <Divider />

        <div className="create-form">
          <div className="form-item">
            <label>工作日期</label>
            <DatePicker
              value={selectedDate}
              onChange={(date) => setSelectedDate(date || dayjs())}
              format="YYYY年MM月DD日"
              allowClear={false}
              style={{ width: '100%' }}
              suffixIcon={<CalendarOutlined />}
            />
          </div>

          <div className="form-item">
            <label>工作内容</label>
            <TextArea
              value={workContent}
              onChange={(e) => setWorkContent(e.target.value)}
              placeholder="请输入工作内容，例如：项目进度汇报、会议纪要、系统维护..."
              rows={3}
              maxLength={50}
              showCount
            />
          </div>

          {folderPreview && (
            <div className="folder-preview">
              <label>文件夹名称预览</label>
              <div className="preview-name">
                <FolderAddOutlined />
                <span>{folderPreview}</span>
              </div>
            </div>
          )}

          <div className="form-actions">
            <Button
              type="primary"
              size="large"
              icon={<FolderAddOutlined />}
              onClick={handleCreate}
              loading={creating}
              disabled={!workContent.trim()}
              block
            >
              创建文件夹
            </Button>
          </div>
        </div>
      </Card>

      {/* 部门选择弹窗 */}
      <DepartmentSelectModal
        open={deptModalOpen}
        mail={null}
        onConfirm={handleDeptConfirm}
        onCancel={() => setDeptModalOpen(false)}
        title="选择所属部门"
        description="选择该工作任务所属的部门，用于后续归档"
      />
    </div>
  )
}

export default QuickCreate
