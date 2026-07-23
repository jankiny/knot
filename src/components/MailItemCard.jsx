import { Button, Card, Collapse, Tag, Tooltip } from 'antd'
import { CheckCircleOutlined, EyeOutlined, FolderAddOutlined, InboxOutlined, PaperClipOutlined } from '@ant-design/icons'
import { formatFileSize, formatMailDate, getBodyPreview } from './mailListUtils'

function MailStatusTag({ status }) {
  if (status === 'archived') {
    return (
      <Tag icon={<InboxOutlined />} color="default">
        已归档
      </Tag>
    )
  }
  if (status === 'working') {
    return (
      <Tag icon={<CheckCircleOutlined />} color="success">
        已生成
      </Tag>
    )
  }
  return null
}

function MailItemCard({
  mail,
  status,
  creating,
  loadingDetail,
  onPreview,
  onCreate
}) {
  const hasAttachments = mail.attachment_count > 0 || mail.has_attachments

  return (
    <Card className="mail-item" key={mail.id} id={`mail-${mail.id}`}>
      <div className="mail-content">
        <div className="mail-info">
          <div className="mail-subject">{mail.subject}</div>
          <div className="mail-meta">
            <span className="mail-from">{mail.from}</span>
            <span className="mail-date">{formatMailDate(mail.date)}</span>
          </div>
          {mail.body && (
            <div className="mail-body-preview">
              {getBodyPreview(mail.body)}
            </div>
          )}
        </div>

        <div className="mail-actions">
          <MailStatusTag status={status} />

          {hasAttachments && (
            <Tag icon={<PaperClipOutlined />} color="blue">
              {mail.attachment_count > 0 ? `${mail.attachment_count} 个附件` : '有附件'}
            </Tag>
          )}

          <Tooltip title="预览邮件">
            <Button
              icon={<EyeOutlined />}
              onClick={() => onPreview(mail)}
              loading={loadingDetail}
            />
          </Tooltip>

          <Tooltip title={hasAttachments ? '创建文件夹并下载附件' : '创建文件夹'}>
            <Button
              type="primary"
              icon={<FolderAddOutlined />}
              onClick={() => onCreate(mail)}
              loading={creating}
            >
              生成
            </Button>
          </Tooltip>
        </div>
      </div>

      {mail.attachments && mail.attachments.length > 0 && (
        <Collapse
          ghost
          className="attachments-collapse"
          items={[{
            key: '1',
            label: '查看附件详情',
            children: (
              <ul className="attachment-list">
                {mail.attachments.map((att, idx) => (
                  <li key={idx}>
                    <PaperClipOutlined />
                    <span className="att-name">{att.filename}</span>
                    <span className="att-size">{formatFileSize(att.size)}</span>
                  </li>
                ))}
              </ul>
            )
          }]}
        />
      )}
    </Card>
  )
}

export default MailItemCard
