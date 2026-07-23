import {
  addProject,
  deleteProject,
  getProjects,
  getSettings,
  setDefaultProject,
  updateProject
} from '../services/settings'
import OwnershipManager from './OwnershipManager'

function ProjectManager({ onUpdate }) {
  return (
    <OwnershipManager
      onUpdate={onUpdate}
      config={{
        addItem: (name, archivePath) => addProject(name, archivePath, false),
        archiveTagColor: 'default',
        archiveTagText: '直接归档',
        deleteItem: deleteProject,
        getDefaultId: () => getSettings().defaultProjectId,
        getItems: getProjects,
        label: '项目',
        listTitle: '项目列表',
        namePlaceholder: '项目名称，如 大培训',
        pathPlaceholder: '归档路径，如 D:/归档/大培训',
        setDefaultItem: setDefaultProject,
        updateItem: updateProject,
        useYearFolder: false
      }}
    />
  )
}

export default ProjectManager
