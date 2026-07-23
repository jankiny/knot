import { Button, Modal } from 'antd'
import { FolderAddOutlined, PaperClipOutlined } from '@ant-design/icons'
import { formatFileSize, formatMailDate } from './mailListUtils'

function MailPreviewModal({ mail, onClose, onCreate }) {
  return (
    <Modal
      title={mail?.subject}
      open={!!mail}
      onCancel={onClose}
      footer={[
        <Button key="close" onClick={onClose}>
          关闭
        </Button>,
        <Button
          key="create"
          type="primary"
          icon={<FolderAddOutlined />}
          onClick={() => onCreate(mail)}
        >
          生成文件夹
        </Button>
      ]}
      width={700}
    >
      {mail && (
        <div className="mail-preview">
          <div className="preview-header">
            <div className="preview-meta">
              <span className="label">发件人：</span>
              <span className="value">{mail.from}</span>
            </div>
            <div className="preview-meta">
              <span className="label">日期：</span>
              <span className="value">{formatMailDate(mail.date)}</span>
            </div>
            {mail.attachment_count > 0 && (
              <div className="preview-meta">
                <span className="label">附件：</span>
                <span className="value">{mail.attachment_count} 个</span>
              </div>
            )}
          </div>
          <div className="preview-body">
            {mail.body || '(无正文内容)'}
          </div>
          {mail.attachments && mail.attachments.length > 0 && (
            <div className="preview-attachments">
              <div className="attachments-title">附件列表：</div>
              <ul>
                {mail.attachments.map((att, idx) => (
                  <li key={idx}>
                    <PaperClipOutlined /> {att.filename} ({formatFileSize(att.size)})
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </Modal>
  )
}

export default MailPreviewModal
