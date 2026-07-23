import { Tooltip } from 'antd'
import { QuestionCircleOutlined } from '@ant-design/icons'
import { getSettings } from '../../services/settings'
import DepartmentManager from '../DepartmentManager'
import ProjectManager from '../ProjectManager'

function ArchiveSettingsSection({ onSettingsChange }) {
  const refreshSettings = () => {
    onSettingsChange(getSettings())
  }

  return (
    <div id="archive-settings" className="settings-block">
      <h2>归档设置</h2>
      <div className="settings-section">
        <div className="section-header">
          <h3>
            部门管理
            <Tooltip title="部门归档固定按年份分组。">
              <QuestionCircleOutlined className="settings-help-icon" />
            </Tooltip>
          </h3>
        </div>
        <DepartmentManager onUpdate={refreshSettings} />
      </div>

      <div className="settings-section" style={{ marginTop: 24 }}>
        <div className="section-header">
          <h3>
            项目管理
            <Tooltip title="项目归档固定直接进入项目目标文件夹，不再额外按年份分组。">
              <QuestionCircleOutlined className="settings-help-icon" />
            </Tooltip>
          </h3>
        </div>
        <ProjectManager onUpdate={refreshSettings} />
      </div>
    </div>
  )
}

export default ArchiveSettingsSection
