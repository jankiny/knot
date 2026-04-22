import { describe, expect, it } from 'vitest'
import { normalizePathKey, optimizeRecursiveScanDirectories } from './path'

describe('normalizePathKey', () => {
  it('normalizes slash, trailing separator and case for windows-like paths', () => {
    expect(normalizePathKey('C:\\Workspace\\Task\\')).toBe('c:/workspace/task')
    expect(normalizePathKey('c:/workspace/task')).toBe('c:/workspace/task')
  })
})

describe('optimizeRecursiveScanDirectories', () => {
  it('skips duplicated and child directories when parent exists', () => {
    const result = optimizeRecursiveScanDirectories([
      { id: 'a', path: 'C:/Workspace', checked: true },
      { id: 'b', path: 'C:/Workspace/ProjectA', checked: true },
      { id: 'c', path: 'c:\\workspace\\', checked: true },
      { id: 'd', path: 'D:/Archive', checked: true },
      { id: 'e', path: 'D:/Archive/2026', checked: true }
    ])

    const keys = result.map((item) => normalizePathKey(item.path)).sort()
    expect(keys).toEqual(['c:/workspace', 'd:/archive'])
  })
})
