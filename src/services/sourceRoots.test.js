import { describe, expect, it } from 'vitest'
import { buildLegacySourceImport, sourceRootToInput } from './sourceRoots'

describe('legacy source root import payload', () => {
  it('includes only source paths and ownership relationships', () => {
    const payload = buildLegacySourceImport({
      folderPath: 'C:/Work',
      scanPath: 'C:/Active',
      mailPasswordEncrypted: 'secret',
      aiApiKeyEncrypted: 'secret',
      windowStyle: 'classic',
      departments: [
        {
          id: 'dept-1',
          name: '办公室',
          archivePath: 'D:/Archive/Office',
          useYearFolder: true
        }
      ],
      projects: [
        {
          id: 'project-1',
          name: '平台建设',
          archivePath: 'D:/Archive/Platform',
          useYearFolder: false
        }
      ]
    })

    expect(payload).toEqual({
      folder_path: 'C:/Work',
      scan_path: 'C:/Active',
      departments: [
        {
          id: 'dept-1',
          name: '办公室',
          archive_path: 'D:/Archive/Office'
        }
      ],
      projects: [
        {
          id: 'project-1',
          name: '平台建设',
          archive_path: 'D:/Archive/Platform'
        }
      ]
    })
    expect(JSON.stringify(payload)).not.toContain('secret')
    expect(JSON.stringify(payload)).not.toContain('windowStyle')
  })

  it('strips server-managed fields from update input', () => {
    expect(sourceRootToInput({
      id: 'root-1',
      name: '工作日志',
      kind: 'journal',
      path: 'C:/Journal',
      scope_type: 'global',
      scope_id: 'ignored',
      scope_name: 'ignored',
      enabled: false,
      recursive: false,
      local_access: 'read',
      ai_access: 'metadata',
      availability: 'offline'
    })).toEqual({
      name: '工作日志',
      kind: 'journal',
      path: 'C:/Journal',
      scope_type: 'global',
      scope_id: null,
      scope_name: null,
      enabled: false,
      recursive: false,
      local_access: 'read',
      ai_access: 'metadata'
    })
  })
})
