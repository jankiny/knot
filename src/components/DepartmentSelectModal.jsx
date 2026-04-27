import { useEffect, useMemo, useState } from 'react'
import { Button, Empty, Modal, Select, Tag, message } from 'antd'
import {
  getDefaultDepartment,
  getDefaultProject,
  getDepartments,
  getProjects
} from '../services/settings'
import './DepartmentSelectModal.css'

function DepartmentSelectModal({ open, mail, onConfirm, onCancel, title, description }) {
  const [targets, setTargets] = useState([])
  const [selectedTargetId, setSelectedTargetId] = useState(null)

  useEffect(() => {
    if (open) {
      const depts = getDepartments().map((item) => ({
        ...item,
        type: 'department',
        key: `department:${item.id}`,
        typeLabel: '部门'
      }))
      const projects = getProjects().map((item) => ({
        ...item,
        type: 'project',
        key: `project:${item.id}`,
        typeLabel: '项目'
      }))
      const nextTargets = [...depts, ...projects]
      setTargets(nextTargets)

      const defaultDept = getDefaultDepartment()
      const defaultProject = getDefaultProject()
      if (defaultDept) {
        setSelectedTargetId(`department:${defaultDept.id}`)
      } else if (defaultProject) {
        setSelectedTargetId(`project:${defaultProject.id}`)
      } else if (nextTargets.length > 0) {
        setSelectedTargetId(nextTargets[0].key)
      } else {
        setSelectedTargetId(null)
      }
    }
  }, [open])

  const options = useMemo(() => targets.map((target) => ({
    label: target.name,
    value: target.key,
    desc: target.archivePath,
    type: target.type,
    typeLabel: target.typeLabel
  })), [targets])

  const handleConfirm = () => {
    if (!selectedTargetId) {
      message.warning('请选择归属')
      return
    }
    const selectedTarget = targets.find((target) => target.key === selectedTargetId)
    onConfirm(selectedTarget || null)
  }

  const handleSkip = () => {
    onConfirm(null)
  }

  return (
    <Modal
      title={title || '选择归属'}
      open={open}
      onCancel={onCancel}
      footer={[
        <Button key="cancel" onClick={onCancel}>
          取消
        </Button>,
        <Button key="skip" onClick={handleSkip}>
          暂不填写
        </Button>,
        <Button key="confirm" type="primary" onClick={handleConfirm} disabled={targets.length === 0}>
          确定
        </Button>
      ]}
      width={400}
    >
      <div className="dept-select-modal">
        {description && (
          <p className="modal-description">{description}</p>
        )}
        {mail && (
          <div className="mail-info-summary">
            <div className="summary-item">
              <span className="label">邮件主题：</span>
              <span className="value">{mail.subject}</span>
            </div>
            <div className="summary-item">
              <span className="label">发件人：</span>
              <span className="value">{mail.from}</span>
            </div>
          </div>
        )}

        {targets.length === 0 ? (
          <Empty
            description="暂无部门或项目，请先在设置中添加归档目标"
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          />
        ) : (
          <div className="dept-select-area">
            <Select
              style={{ width: '100%' }}
              placeholder="请选择归属"
              value={selectedTargetId}
              onChange={setSelectedTargetId}
              options={options}
              optionRender={(option) => (
                <div className="dept-option">
                  <span className="dept-option-main">
                    <span className="dept-option-name">{option.data.label}</span>
                    <Tag color={option.data.type === 'project' ? 'purple' : 'blue'}>
                      {option.data.typeLabel}
                    </Tag>
                  </span>
                  <span className="dept-option-path">{option.data.desc}</span>
                </div>
              )}
              labelRender={(props) => {
                const target = options.find((option) => option.value === props.value)
                if (!target) return props.label
                return (
                  <span className="dept-select-label">
                    <span>{target.label}</span>
                    <Tag color={target.type === 'project' ? 'purple' : 'blue'}>
                      {target.typeLabel}
                    </Tag>
                  </span>
                )
              }}
            />
            <p className="select-hint">
              部门归档会按年份整理，项目归档会集中到对应项目文件夹。
            </p>
          </div>
        )}
      </div>
    </Modal>
  )
}

export default DepartmentSelectModal
