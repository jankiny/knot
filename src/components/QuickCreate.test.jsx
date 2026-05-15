import { describe, expect, it } from 'vitest'
import dayjs from 'dayjs'
import { buildQuickCreateFolderName, formatQuickCreateDate } from './QuickCreate'

describe('QuickCreate date handling', () => {
  it('keeps manually selected date as a local calendar date', () => {
    const selectedDate = dayjs('2026-04-21')

    expect(formatQuickCreateDate(selectedDate)).toBe('2026-04-21')
  })

  it('uses the selected calendar date in folder preview', () => {
    const folderName = buildQuickCreateFolderName(
      { folderNameFormat: '{{YYYY}}.{{MM}}.{{DD}}_{{subject}}' },
      '补建工作',
      dayjs('2026-04-21')
    )

    expect(folderName).toBe('2026.04.21_补建工作')
  })
})
