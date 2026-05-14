import { useEffect, useMemo, useState } from 'react'
import { Button, Empty, Modal, Select, Tag, message } from 'antd'
import { sopApi } from '../services/api'
import {
  getDefaultDepartment,
  getDefaultProject,
  getDefaultSopTemplateId,
  getDepartments,
  getProjects
} from '../services/settings'
import './DepartmentSelectModal.css'

function DepartmentSelectModal({ open, mail, onConfirm, onCancel, title, description, enableSop = false }) {
  const [targets, setTargets] = useState([])
  const [selectedTargetId, setSelectedTargetId] = useState(null)
  const [templates, setTemplates] = useState([])
  const [selectedTemplateId, setSelectedTemplateId] = useState(getDefaultSopTemplateId())

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

      if (enableSop) {
        const defaultTemplateId = getDefaultSopTemplateId()
        setSelectedTemplateId(defaultTemplateId)
        sopApi.listTemplates()
          .then((result) => {
            const nextTemplates = result.templates || []
            setTemplates(nextTemplates)
            if (!nextTemplates.some((tpl) => tpl.id === defaultTemplateId)) {
              setSelectedTemplateId('default-task')
            }
          })
          .catch(() => setTemplates([]))
      }
    }
  }, [open, enableSop])

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
    onConfirm(selectedTarget || null, enableSop ? selectedTemplateId : null)
  }

  const handleSkip = () => {
    onConfirm(null, enableSop ? selectedTemplateId : null)
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

        {enableSop && (
          <div className="dept-select-area">
            <Select
              style={{ width: '100%' }}
              placeholder="请选择标准流程"
              value={selectedTemplateId}
              onChange={setSelectedTemplateId}
              options={(templates.length ? templates : [{ id: 'default-task', name: '通用任务' }]).map((tpl) => ({
                label: tpl.name,
                value: tpl.id,
                desc: tpl.description,
                typeLabel: tpl.builtin ? '内置' : '已安装'
              }))}
              optionRender={(option) => (
                <div className="dept-option">
                  <span className="dept-option-main">
                    <span className="dept-option-name">{option.data.label}</span>
                    <Tag color={option.data.typeLabel === '内置' ? 'blue' : 'green'}>
                      {option.data.typeLabel}
                    </Tag>
                  </span>
                  <span className="dept-option-path">{option.data.desc}</span>
                </div>
              )}
            />
            <p className="select-hint">
              标准流程会决定新任务的目录结构和初始模板文件。
            </p>
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
