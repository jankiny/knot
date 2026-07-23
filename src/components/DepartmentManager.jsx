import {
  addDepartment,
  deleteDepartment,
  getDepartments,
  getSettings,
  setDefaultDepartment,
  updateDepartment
} from '../services/settings'
import OwnershipManager from './OwnershipManager'

function DepartmentManager({ onUpdate }) {
  return (
    <OwnershipManager
      onUpdate={onUpdate}
      config={{
        addItem: (name, archivePath) => addDepartment(name, archivePath, true),
        archiveTagColor: 'blue',
        archiveTagText: '按年份',
        deleteItem: deleteDepartment,
        getDefaultId: () => getSettings().defaultDepartmentId,
        getItems: getDepartments,
        label: '部门',
        listTitle: '部门列表',
        namePlaceholder: '部门名称',
        pathPlaceholder: '归档路径，如 D:/归档/教务部',
        setDefaultItem: setDefaultDepartment,
        updateItem: updateDepartment,
        useYearFolder: true
      }}
    />
  )
}

export default DepartmentManager
