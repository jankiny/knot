import { describe, expect, it } from 'vitest'
import {
  DEFAULT_PERSONAL_SUMMARY_QUERY,
  approvedAIDestination,
  buildManifestSummary,
  summarizeExclusions
} from './PersonalSummary'

describe('PersonalSummary helpers', () => {
  it('keeps the default request free of paths', () => {
    expect(DEFAULT_PERSONAL_SUMMARY_QUERY).not.toMatch(/[A-Za-z]:[\\/]/)
    expect(DEFAULT_PERSONAL_SUMMARY_QUERY).not.toContain('scan_path')
    expect(DEFAULT_PERSONAL_SUMMARY_QUERY).toContain('1500 字')
  })

  it('summarizes journal, project, source, and evidence counts', () => {
    expect(buildManifestSummary({
      sources: [
        { source_root_id: 'journal', scope_type: 'global' },
        { source_root_id: 'project', scope_type: 'project', scope_name: 'Knot' }
      ],
      evidence: [
        { source_type: 'journal_daily', project: '' },
        { source_type: 'journal_weekly', project: 'Knot' },
        { source_type: 'work_record', project: '另一个项目' }
      ]
    })).toEqual({
      journalCount: 2,
      projectCount: 2,
      evidenceCount: 3,
      sourceCount: 2
    })
  })

  it('groups visible exclusions and keeps omitted count explicit', () => {
    expect(summarizeExclusions({
      excluded: [
        { reason_code: 'ai_access_denied' },
        { reason_code: 'outside_period' },
        { reason_code: 'ai_access_denied' }
      ],
      omitted_excluded_count: 4
    })).toEqual([
      { code: 'omitted', count: 4 },
      { code: 'ai_access_denied', count: 2 },
      { code: 'outside_period', count: 1 }
    ])
  })

  it('accepts only approved official HTTPS model destinations', () => {
    expect(approvedAIDestination('https://api.deepseek.com')).toBe('api.deepseek.com')
    expect(approvedAIDestination('https://api.openai.com/v1')).toBe('api.openai.com')
    expect(approvedAIDestination('http://api.deepseek.com')).toBe('')
    expect(approvedAIDestination('https://api.deepseek.com.evil.invalid')).toBe('')
    expect(approvedAIDestination('https://example.invalid/v1')).toBe('')
  })
})
