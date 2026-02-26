import { useState, useEffect } from 'react'
import { Modal, Select, Empty, Button, message } from 'antd'
import { getDepartments, getDefaultDepartment } from '../services/settings'
import './DepartmentSelectModal.css'

function DepartmentSelectModal({ open, mail, onConfirm, onCancel, title, description }) {
  const [departments, setDepartments] = useState([])
  const [selectedDeptId, setSelectedDeptId] = useState(null)

  useEffect(() => {
    if (open) {
      const depts = getDepartments()
      setDepartments(depts)
      // 默认选中默认部门
      const defaultDept = getDefaultDepartment()
      if (defaultDept) {
        setSelectedDeptId(defaultDept.id)
      } else if (depts.length > 0) {
        setSelectedDeptId(depts[0].id)
      }
    }
  }, [open])

  const handleConfirm = () => {
    if (!selectedDeptId) {
      message.warning('请选择部门')
      return
    }
    const selectedDept = departments.find(d => d.id === selectedDeptId)
    onConfirm(selectedDept)
  }

  const handleSkip = () => {
    // 暂不填写归属部门，仍会生成工作记录.md（部门留空）
    onConfirm(null)
  }

  return (
    <Modal
      title={title || "选择归属部门"}
      open={open}
      onCancel={onCancel}
      footer={[
        <Button key="cancel" onClick={onCancel}>
          取消
        </Button>,
        <Button key="skip" onClick={handleSkip}>
          暂不填写
        </Button>,
        <Button key="confirm" type="primary" onClick={handleConfirm} disabled={departments.length === 0}>
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

        {departments.length === 0 ? (
          <Empty
            description="暂无部门，请先在设置中添加部门"
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          />
        ) : (
          <div className="dept-select-area">
            <label className="select-label">选择部门：</label>
            <Select
              style={{ width: '100%' }}
              placeholder="请选择部门"
              value={selectedDeptId}
              onChange={setSelectedDeptId}
              options={departments.map(d => ({
                label: d.name,
                value: d.id,
                desc: d.archivePath
              }))}
              optionRender={(option) => (
                <div className="dept-option">
                  <span className="dept-option-name">{option.data.label}</span>
                  <span className="dept-option-path">{option.data.desc}</span>
                </div>
              )}
            />
            <p className="select-hint">
              选择部门后，将写入"工作记录.md"的归属部门字段，便于后续归档
            </p>
          </div>
        )}
      </div>
    </Modal>
  )
}

export default DepartmentSelectModal
