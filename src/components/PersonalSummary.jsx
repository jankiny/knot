import { useMemo, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Col,
  DatePicker,
  Empty,
  Input,
  List,
  message,
  Progress,
  Row,
  Space,
  Statistic,
  Tag,
  Tooltip,
  Typography
} from 'antd'
import {
  CopyOutlined,
  FileSearchOutlined,
  SafetyCertificateOutlined,
  SendOutlined
} from '@ant-design/icons'
import dayjs from 'dayjs'
import { contextApi, sourceRootApi } from '../services/api'
import { getReportAiConfig } from '../hooks/useAiConfig'
import { getSelectedAiModel } from '../services/settings'
import './PersonalSummary.css'

const { Paragraph, Text, Title } = Typography
const APPROVED_AI_HOSTS = new Set(['api.deepseek.com', 'api.openai.com'])

export const DEFAULT_PERSONAL_SUMMARY_QUERY = '我已经工作一年了，请根据现有工作资料生成一份 1500 字左右个人工作总结大纲。'

const SOURCE_KIND_LABELS = {
  journal: '工作日志',
  current_work: '当前工作',
  active_work: '扫描工作',
  work_archive: '工作归档'
}

const EXCLUSION_LABELS = {
  outside_period: '不在时间范围',
  root_disabled: '资料源已停用',
  root_offline: '资料源离线',
  local_access_denied: '本地读取受限',
  ai_access_denied: '禁止用于 AI',
  sensitive_path_marker: '敏感路径标记',
  metadata_only: '仅允许元数据',
  index_stale: '索引已过期',
  index_deleted: '文件已移动或删除',
  index_error: '索引错误',
  index_outdated: '文件有变化，需重扫',
  read_error: '文件不可读',
  token_budget: '超出 evidence/token 预算',
  candidate_limit: '超过候选上限'
}

export function approvedAIDestination(apiUrl) {
  try {
    const parsed = new URL(String(apiUrl || '').trim())
    const host = parsed.hostname.toLowerCase()
    if (
      parsed.protocol !== 'https:' ||
      parsed.username ||
      parsed.password ||
      (parsed.port && parsed.port !== '443') ||
      !APPROVED_AI_HOSTS.has(host)
    ) {
      return ''
    }
    return host
  } catch {
    return ''
  }
}

export function buildManifestSummary(manifest) {
  const evidence = manifest?.evidence || []
  const projects = new Set()
  let journalCount = 0
  evidence.forEach((item) => {
    if (String(item.source_type || '').startsWith('journal_')) {
      journalCount += 1
    }
    const project = String(item.project || '').trim()
    if (project) projects.add(project)
  })
  ;(manifest?.sources || []).forEach((source) => {
    if (source.scope_type === 'project') {
      const project = String(source.scope_name || '').trim()
      if (project) projects.add(project)
    }
  })
  return {
    journalCount,
    projectCount: projects.size,
    evidenceCount: evidence.length,
    sourceCount: (manifest?.sources || []).length
  }
}

export function summarizeExclusions(manifest) {
  const counts = new Map()
  ;(manifest?.excluded || []).forEach((item) => {
    const code = item.reason_code || 'other'
    counts.set(code, (counts.get(code) || 0) + 1)
  })
  const omitted = Number(manifest?.omitted_excluded_count || 0)
  if (omitted > 0) counts.set('omitted', omitted)
  return Array.from(counts, ([code, count]) => ({ code, count }))
    .sort((left, right) => right.count - left.count || left.code.localeCompare(right.code))
}

function EvidenceReferences({ evidenceById, ids }) {
  return (
    <Space size={[4, 4]} wrap>
      {(ids || []).map((id) => {
        const evidence = evidenceById.get(id)
        return (
          <Tooltip
            key={id}
            title={evidence ? `${evidence.title}${evidence.date ? ` · ${evidence.date}` : ''}` : id}
          >
            <Tag color="blue">{id}</Tag>
          </Tooltip>
        )
      })}
    </Space>
  )
}

function PersonalSummary() {
  const [query, setQuery] = useState(DEFAULT_PERSONAL_SUMMARY_QUERY)
  const [period, setPeriod] = useState(() => {
    const today = dayjs().startOf('day')
    return [today.subtract(1, 'year'), today]
  })
  const [discoverLoading, setDiscoverLoading] = useState(false)
  const [generateLoading, setGenerateLoading] = useState(false)
  const [manifest, setManifest] = useState(null)
  const [selectedEvidenceIDs, setSelectedEvidenceIDs] = useState({})
  const [scanStatus, setScanStatus] = useState('')
  const [sendConfirmed, setSendConfirmed] = useState(false)
  const [generation, setGeneration] = useState(null)

  const selectedModel = useMemo(() => getSelectedAiModel(), [])
  const destination = approvedAIDestination(selectedModel?.apiUrl)
  const selectedEvidence = useMemo(
    () => (manifest?.evidence || []).filter((item) => selectedEvidenceIDs[item.id]),
    [manifest, selectedEvidenceIDs]
  )
  const manifestSummary = useMemo(() => buildManifestSummary(manifest), [manifest])
  const exclusionSummary = useMemo(() => summarizeExclusions(manifest), [manifest])
  const evidenceById = useMemo(
    () => new Map((manifest?.evidence || []).map((item) => [item.id, item])),
    [manifest]
  )

  const resetGeneratedState = () => {
    setSendConfirmed(false)
    setGeneration(null)
  }

  const handlePeriodChange = (value) => {
    setPeriod(value || [])
    setManifest(null)
    setSelectedEvidenceIDs({})
    resetGeneratedState()
  }

  const handleDiscover = async () => {
    if (!query.trim()) {
      message.warning('请输入个人总结需求')
      return
    }
    if (period.length !== 2 || !period[0] || !period[1] || period[1].isBefore(period[0], 'day')) {
      message.warning('请选择有效的总结时间范围')
      return
    }

    setDiscoverLoading(true)
    setManifest(null)
    setSelectedEvidenceIDs({})
    setScanStatus('')
    resetGeneratedState()
    try {
      try {
        const scan = await sourceRootApi.scanAll(false)
        const results = scan.results || []
        const filesSeen = results.reduce((total, item) => total + Number(item.files_seen || 0), 0)
        setScanStatus(`索引更新：${scan.source_roots || results.length} 个资料源，检查 ${filesSeen} 个文件`)
      } catch (error) {
        setScanStatus('索引更新未完成，已使用现有索引继续发现')
        message.warning(error.response?.data?.detail || '资料源扫描未完成，继续使用现有索引')
      }

      const discovered = await contextApi.discover({
        task_type: 'personal_annual_summary',
        query: query.trim(),
        period_start: period[0].format('YYYY-MM-DD'),
        period_end: period[1].format('YYYY-MM-DD')
      })
      setManifest(discovered)
      setSelectedEvidenceIDs(Object.fromEntries(
        (discovered.evidence || []).map((item) => [item.id, true])
      ))
      if ((discovered.evidence || []).length === 0) {
        message.info('没有发现可用于生成的 evidence，请检查资料源、索引和时间范围')
      } else {
        message.success(`发现 ${discovered.evidence.length} 条可确认 evidence`)
      }
    } catch (error) {
      message.error(error.response?.data?.detail || '发现总结资料失败')
    } finally {
      setDiscoverLoading(false)
    }
  }

  const handleGenerate = async () => {
    if (!manifest?.id || !manifest.valid || manifest.requires_rediscovery) {
      message.warning('当前资料清单已失效，请重新发现')
      return
    }
    if (selectedEvidence.length === 0) {
      message.warning('请至少保留一条 evidence')
      return
    }
    if (!sendConfirmed) {
      message.warning('请先确认发送范围和模型目标')
      return
    }
    if (!destination) {
      message.warning('个人总结首版仅支持官方 DeepSeek/OpenAI HTTPS 端点')
      return
    }

    setGenerateLoading(true)
    try {
      const ai = await getReportAiConfig({ featureLabel: '个人总结生成' })
      if (!ai) return
      if (approvedAIDestination(ai.api_url) !== destination) {
        message.error('模型目标已变化，请重新确认发送范围')
        setSendConfirmed(false)
        return
      }
      const response = await contextApi.generate(manifest.id, {
        evidence_ids: selectedEvidence.map((item) => item.id),
        confirm_send: true,
        ai
      })
      setGeneration(response)
      message.success('个人年度总结大纲已生成')
    } catch (error) {
      if (error.response?.status === 409) {
        setSendConfirmed(false)
        message.error('资料清单已过期或发生变化，请重新发现后再生成')
      } else {
        const runID = error.response?.data?.ai_run_id
        const suffix = runID ? `（审计 ID：${runID}）` : ''
        message.error(`${error.response?.data?.detail || '生成个人总结失败'}${suffix}`)
      }
    } finally {
      setGenerateLoading(false)
    }
  }

  const handleCopy = async () => {
    const markdown = generation?.result?.markdown || ''
    if (!markdown.trim()) {
      message.info('暂无可复制内容')
      return
    }
    try {
      await navigator.clipboard.writeText(markdown)
      message.success('已复制 Markdown')
    } catch {
      message.error('复制失败')
    }
  }

  const setAllEvidence = (checked) => {
    setSelectedEvidenceIDs(Object.fromEntries(
      (manifest?.evidence || []).map((item) => [item.id, checked])
    ))
    resetGeneratedState()
  }

  const result = generation?.result

  return (
    <div className="personal-summary">
      <Card title="总结需求与时间范围" className="personal-summary-card">
        <div className="personal-summary-form">
          <div className="personal-summary-query">
            <div className="personal-summary-label">总结需求</div>
            <Input.TextArea
              value={query}
              onChange={(event) => {
                setQuery(event.target.value)
                setManifest(null)
                resetGeneratedState()
              }}
              autoSize={{ minRows: 2, maxRows: 4 }}
              maxLength={2000}
              showCount
            />
          </div>
          <div>
            <div className="personal-summary-label">总结周期</div>
            <DatePicker.RangePicker
              value={period}
              onChange={handlePeriodChange}
              allowClear={false}
            />
          </div>
        </div>
        <div className="personal-summary-actions">
          {scanStatus ? <Text type="secondary">{scanStatus}</Text> : <span />}
          <Button
            type="primary"
            icon={<FileSearchOutlined />}
            loading={discoverLoading}
            onClick={handleDiscover}
          >
            更新索引并发现资料
          </Button>
        </div>
      </Card>

      <Card title="资料范围预览" className="personal-summary-card">
        {!manifest ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={discoverLoading ? '正在更新索引并发现资料…' : '先发现资料，再确认发送范围'}
          />
        ) : (
          <>
            <Row gutter={[12, 12]} className="personal-summary-statistics">
              <Col xs={12} md={6}><Statistic title="已发现日志" value={manifestSummary.journalCount} /></Col>
              <Col xs={12} md={6}><Statistic title="已发现项目" value={manifestSummary.projectCount} /></Col>
              <Col xs={12} md={6}><Statistic title="资料源范围" value={manifestSummary.sourceCount} /></Col>
              <Col xs={12} md={6}><Statistic title="候选 evidence" value={manifestSummary.evidenceCount} /></Col>
            </Row>

            <div className="personal-summary-section">
              <div className="personal-summary-section-title">资料源范围</div>
              <Space size={[6, 6]} wrap>
                {(manifest.sources || []).map((source) => (
                  <Tag key={source.source_root_id} color="blue">
                    {source.name || SOURCE_KIND_LABELS[source.kind] || source.kind}
                    {' · '}
                    {source.evidence_items}/{source.candidate_documents}
                  </Tag>
                ))}
                {(manifest.sources || []).length === 0 && <Text type="secondary">没有可用资料源</Text>}
              </Space>
            </div>

            <div className="personal-summary-section">
              <div className="personal-summary-section-title">排除原因汇总</div>
              <Space size={[6, 6]} wrap>
                {exclusionSummary.map((item) => (
                  <Tag key={item.code}>
                    {item.code === 'omitted' ? '未展开的排除项' : (EXCLUSION_LABELS[item.code] || item.code)}
                    {' '}
                    {item.count}
                  </Tag>
                ))}
                {exclusionSummary.length === 0 && <Text type="secondary">没有排除项</Text>}
              </Space>
            </div>

            {(manifest.unavailable_sources || []).map((source) => (
              <Alert
                key={source.source_root_id}
                className="personal-summary-alert"
                type="warning"
                showIcon
                message={`${source.name} 当前不可用`}
                description={`${source.reason} 生成结果会把这部分资料标记为信息缺口。`}
              />
            ))}

            <div className="personal-summary-token">
              <div>
                <div className="personal-summary-section-title">预计模型输入</div>
                <Text type="secondary">
                  {manifest.estimated_input_tokens} / {manifest.token_budget} tokens
                </Text>
              </div>
              <Progress
                percent={Math.min(100, Math.round(
                  (Number(manifest.estimated_input_tokens || 0) / Math.max(1, Number(manifest.token_budget || 1))) * 100
                ))}
                showInfo={false}
              />
            </div>
          </>
        )}
      </Card>

      <Card
        title="Evidence 确认"
        className="personal-summary-card"
        extra={manifest ? (
          <Space>
            <Tag color="green">保留 {selectedEvidence.length} 条</Tag>
            <Button size="small" onClick={() => setAllEvidence(true)}>全选</Button>
            <Button size="small" onClick={() => setAllEvidence(false)}>清空</Button>
          </Space>
        ) : null}
      >
        {!manifest?.evidence?.length ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无可确认 evidence" />
        ) : (
          <List
            className="personal-summary-evidence-list"
            dataSource={manifest.evidence}
            renderItem={(item) => (
              <List.Item>
                <div className="personal-summary-evidence">
                  <Checkbox
                    checked={!!selectedEvidenceIDs[item.id]}
                    onChange={(event) => {
                      setSelectedEvidenceIDs((previous) => ({
                        ...previous,
                        [item.id]: event.target.checked
                      }))
                      resetGeneratedState()
                    }}
                  >
                    <span className="personal-summary-evidence-title">{item.title || '未命名 evidence'}</span>
                  </Checkbox>
                  <div className="personal-summary-evidence-meta">
                    <Tag>{item.date || '日期未知'}</Tag>
                    <Tag color="cyan">{item.source_type}</Tag>
                    {item.project && <Tag color="purple">{item.project}</Tag>}
                    <Tag color={item.ai_access_effective === 'content' ? 'green' : 'gold'}>
                      {item.ai_access_effective === 'content' ? '允许受限正文' : '仅元数据'}
                    </Tag>
                    <Tag>{item.id}</Tag>
                  </div>
                  {item.excerpt ? (
                    <Paragraph ellipsis={{ rows: 3, expandable: true, symbol: '展开' }}>
                      {item.excerpt}
                    </Paragraph>
                  ) : (
                    <Text type="secondary">此 evidence 不含正文，模型只能使用允许的元数据。</Text>
                  )}
                  <div className="personal-summary-evidence-reason">{item.reason}</div>
                </div>
              </List.Item>
            )}
          />
        )}

        <div className="personal-summary-confirm">
          {destination ? (
            <Alert
              type="info"
              showIcon
              icon={<SafetyCertificateOutlined />}
              message={`当前模型：${selectedModel?.name || selectedModel?.modelId || '未配置'}`}
              description={`确认后，仅将勾选且重新校验有效的 evidence 发送到 ${destination}；不会发送排除项、离线资料或路径。API Key 仅用于请求鉴权，不进入 prompt 或审计。`}
            />
          ) : (
            <Alert
              type="warning"
              showIcon
              message="当前模型目标不在阶段 5 允许范围"
              description="个人总结首版仅支持 api.deepseek.com 或 api.openai.com 的官方 HTTPS 端点；现有其他 AI 功能不受影响。"
            />
          )}
          <Checkbox
            checked={sendConfirmed}
            disabled={!destination || selectedEvidence.length === 0}
            onChange={(event) => setSendConfirmed(event.target.checked)}
          >
            我已检查资料范围，并确认将 {selectedEvidence.length} 条 evidence 发送到 {destination || '受支持的模型目标'}
          </Checkbox>
          <Button
            type="primary"
            icon={<SendOutlined />}
            loading={generateLoading}
            disabled={!sendConfirmed || selectedEvidence.length === 0 || !destination}
            onClick={handleGenerate}
          >
            确认并生成大纲
          </Button>
        </div>
      </Card>

      <Card title="个人年度总结大纲" className="personal-summary-card">
        {!result ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="确认生成后，在这里查看大纲、证据引用和信息缺口" />
        ) : (
          <>
            <div className="personal-summary-output-title">
              <div>
                <Title level={4}>{result.title}</Title>
                <Text type="secondary">
                  目标约 {result.target_word_count} 字 · {generation.evidence_count} 条 evidence · 审计 ID {generation.run_id}
                </Text>
              </div>
              <Button icon={<CopyOutlined />} onClick={handleCopy}>复制 Markdown</Button>
            </div>

            <div className="personal-summary-outline">
              {(result.sections || []).map((section, index) => (
                <div className="personal-summary-outline-section" key={`${section.heading}-${index}`}>
                  <div className="personal-summary-outline-heading">
                    <span>{index + 1}. {section.heading}</span>
                    <Tag>{section.word_count} 字</Tag>
                  </div>
                  <ul>
                    {(section.outline || []).map((item, itemIndex) => (
                      <li key={`${item}-${itemIndex}`}>{item}</li>
                    ))}
                  </ul>
                  <EvidenceReferences evidenceById={evidenceById} ids={section.evidence_ids} />
                </div>
              ))}
            </div>

            <div className="personal-summary-result-grid">
              <div>
                <div className="personal-summary-section-title">可核实成果</div>
                {(result.verified_results || []).length === 0 ? (
                  <Text type="secondary">当前 evidence 未形成可核实成果。</Text>
                ) : (
                  <List
                    size="small"
                    dataSource={result.verified_results}
                    renderItem={(item) => (
                      <List.Item>
                        <div>
                          <div>{item.statement}</div>
                          <EvidenceReferences evidenceById={evidenceById} ids={item.evidence_ids} />
                        </div>
                      </List.Item>
                    )}
                  />
                )}
              </div>
              <div>
                <div className="personal-summary-section-title">信息不足 / 待补充</div>
                {(result.missing_information || []).length === 0 ? (
                  <Text type="secondary">暂无额外待补充项。</Text>
                ) : (
                  <List
                    size="small"
                    dataSource={result.missing_information}
                    renderItem={(item) => <List.Item>{item}</List.Item>}
                  />
                )}
              </div>
            </div>

            <div className="personal-summary-markdown">
              <div className="personal-summary-section-title">Markdown 输出</div>
              <Input.TextArea
                value={result.markdown || ''}
                readOnly
                autoSize={{ minRows: 10, maxRows: 24 }}
              />
            </div>
          </>
        )}
      </Card>
    </div>
  )
}

export default PersonalSummary
