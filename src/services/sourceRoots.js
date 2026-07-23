import { sourceRootApi } from './api'

export function buildLegacySourceImport(settings = {}) {
  return {
    folder_path: settings.folderPath || '',
    scan_path: settings.scanPath || '',
    departments: (settings.departments || []).map((department) => ({
      id: department.id || '',
      name: department.name || '',
      archive_path: department.archivePath || ''
    })),
    projects: (settings.projects || []).map((project) => ({
      id: project.id || '',
      name: project.name || '',
      archive_path: project.archivePath || ''
    }))
  }
}

export function sourceRootToInput(sourceRoot) {
  return {
    name: sourceRoot.name,
    kind: sourceRoot.kind,
    path: sourceRoot.path,
    scope_type: sourceRoot.scope_type || 'global',
    scope_id: sourceRoot.scope_type === 'global' ? null : (sourceRoot.scope_id || null),
    scope_name: sourceRoot.scope_type === 'global' ? null : (sourceRoot.scope_name || null),
    enabled: sourceRoot.enabled !== false,
    recursive: sourceRoot.recursive !== false,
    local_access: sourceRoot.local_access || 'read',
    ai_access: sourceRoot.ai_access || (sourceRoot.kind === 'private' ? 'none' : 'content')
  }
}

export function importLegacySourceRoots(settings) {
  return sourceRootApi.importLegacy(buildLegacySourceImport(settings))
}
