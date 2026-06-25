import { describe, expect, it } from 'vitest'
import { buildMailTimeline, formatFileSize, getBodyPreview, getMailMonthKey } from './mailListUtils'

describe('mailListUtils', () => {
  it('formats attachment sizes', () => {
    expect(formatFileSize(512)).toBe('512 B')
    expect(formatFileSize(1536)).toBe('1.5 KB')
    expect(formatFileSize(2 * 1024 * 1024)).toBe('2.0 MB')
  })

  it('builds compact body previews', () => {
    expect(getBodyPreview('第一行\n\n第二行', 20)).toBe('第一行 第二行')
    expect(getBodyPreview('abcdefghijkl', 5)).toBe('abcde...')
    expect(getBodyPreview('')).toBe('')
  })

  it('extracts month keys from valid mail dates', () => {
    expect(getMailMonthKey('2026-04-21T10:30:00+08:00')).toBe('2026-04')
    expect(getMailMonthKey('not-a-date')).toBe('')
  })

  it('builds timeline entries using the newest mail in each month', () => {
    const timeline = buildMailTimeline([
      { id: 'a', date: '2026-05-20T10:00:00+08:00' },
      { id: 'b', date: '2026-05-01T10:00:00+08:00' },
      { id: 'c', date: '2026-04-21T10:00:00+08:00' },
      { id: 'bad', date: 'invalid' }
    ])

    expect(timeline.map(item => item.key)).toEqual(['2026-05', '2026-04'])
    expect(timeline[0]).toMatchObject({
      label: '2026年5月',
      mailId: 'a'
    })
  })
})
