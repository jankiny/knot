import { describe, expect, it } from 'vitest'
import { formatFolderName } from './settings'

describe('formatFolderName', () => {
  it('formats folder name for mail creation', () => {
    const name = formatFolderName('{{YYYY}}.{{MM}}.{{DD}}_{{subject}}_{{from}}', {
      subject: '项目进度汇报',
      from: '张三 <zhangsan@company.com>',
      date: '2026-04-22 10:00:00'
    })

    expect(name).toBe('2026.04.22_项目进度汇报_张三')
  })

  it('formats folder name for quick creation using work content as subject', () => {
    const name = formatFolderName('{{YYYY}}.{{MM}}.{{DD}}_{{subject}}', {
      subject: '需求:评审/版本1',
      from: '',
      date: '2026-04-20 09:30:00'
    })

    expect(name).toBe('2026.04.20_需求评审版本1')
  })
})
