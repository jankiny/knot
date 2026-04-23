import { useMemo, useState } from 'react'
import { Button, Card, DatePicker, Divider, Input, message, Typography } from 'antd'
import { CalendarOutlined, FolderAddOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { folderApi } from '../services/api'
import { formatFolderName, generateFolderHash, getSettings } from '../services/settings'
import DepartmentSelectModal from './DepartmentSelectModal'
import './QuickCreate.css'

const { Title, Text } = Typography
const { TextArea } = Input

function buildQuickCreateFolderName(settings, workContent, selectedDate) {
  const content = workContent.trim()
  if (!content) return ''

  return formatFolderName(settings.folderNameFormat, {
    subject: content,
    date: selectedDate.toISOString(),
    from: ''
  })
}

function QuickCreate() {
  const [workContent, setWorkContent] = useState('')
  const [selectedDate, setSelectedDate] = useState(dayjs())
  const [creating, setCreating] = useState(false)
  const [deptModalOpen, setDeptModalOpen] = useState(false)

  const folderPreview = useMemo(() => {
    const settings = getSettings()
    return buildQuickCreateFolderName(settings, workContent, selectedDate)
  }, [workContent, selectedDate])

  const handleCreate = () => {
    if (!workContent.trim()) {
      message.warning('请输入工作内容')
      return
    }
    setDeptModalOpen(true)
  }

  const handleDeptConfirm = async (department) => {
    setDeptModalOpen(false)
    await createFolder(department)
  }

  const createFolder = async (department = null) => {
    setCreating(true)
    try {
      const settings = getSettings()
      const folderName = buildQuickCreateFolderName(settings, workContent, selectedDate)
      if (!folderName) {
        message.warning('无法生成文件夹名称，请检查输入')
        return
      }

      const requestData = {
        base_path: settings.folderPath,
        folder_name: folderName,
        mail_id: null,
        subject: workContent.trim(),
        date: selectedDate.toISOString(),
        from_addr: '',
        body: '',
        use_sub_folder: false,
        save_mail_content: false,
        attachments: [],
        department: department ? department.name : null,
        source: 'manual',
        hash: await generateFolderHash(folderName)
      }

      const result = await folderApi.create(requestData)
      message.success(result.message || `文件夹已创建: ${folderName}`)
      setWorkContent('')
      setSelectedDate(dayjs())
    } catch (error) {
      message.error(error.response?.data?.detail || '创建文件夹失败')
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="quick-create">
      <Card variant="borderless">
        <Title level={4}>快速创建工作文件夹</Title>
        <Text type="secondary">用于非邮件来源任务，自动按命名规则生成文件夹并创建标准结构。</Text>

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
              placeholder="请输入工作内容，例如：项目进度汇报、会议纪要、系统维护"
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

      <DepartmentSelectModal
        open={deptModalOpen}
        mail={null}
        onConfirm={handleDeptConfirm}
        onCancel={() => setDeptModalOpen(false)}
        title="选择所属部门"
        description="选择该任务所属部门，用于后续归档。"
      />
    </div>
  )
}

export default QuickCreate
