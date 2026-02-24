package mail

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type MailClient struct {
	server   string
	port     int
	username string
	password string
	useSSL   bool
	conn     *client.Client
}

func NewMailClient(server string, port int, username, password string, useSSL bool) *MailClient {
	return &MailClient{
		server:   server,
		port:     port,
		username: username,
		password: password,
		useSSL:   useSSL,
	}
}

func (c *MailClient) Connect() error {
	addr := fmt.Sprintf("%s:%d", c.server, c.port)
	var err error

	if c.useSSL {
		c.conn, err = client.DialTLS(addr, &tls.Config{InsecureSkipVerify: true})
	} else {
		c.conn, err = client.Dial(addr)
	}

	if err != nil {
		return fmt.Errorf("connect error: %w", err)
	}

	if err := c.conn.Login(c.username, c.password); err != nil {
		return fmt.Errorf("login error: %w", err)
	}

	_, err = c.conn.Select("INBOX", false)
	if err != nil {
		return fmt.Errorf("select inbox error: %w", err)
	}

	return nil
}

func (c *MailClient) Disconnect() {
	if c.conn != nil {
		c.conn.Logout()
		c.conn = nil
	}
}

// MailItem respresents a single list item
type MailItem struct {
	ID              string `json:"id"`
	Subject         string `json:"subject"`
	From            string `json:"from"`
	Date            string `json:"date"`
	AttachmentCount int    `json:"attachment_count"`
	HasAttachments  bool   `json:"has_attachments"`
}

func (c *MailClient) ensureConnection() error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	if err := c.conn.Noop(); err != nil {
		log.Printf("IMAP connection check failed, trying to reconnect: %v", err)
		c.conn = nil
		if err := c.Connect(); err != nil {
			return fmt.Errorf("reconnect failed: %w", err)
		}
	}
	return nil
}

func (c *MailClient) FetchMailList(limit int, days int) ([]MailItem, error) {
	if err := c.ensureConnection(); err != nil {
		return nil, err
	}

	criteria := imap.NewSearchCriteria()
	if days > 0 {
		since := time.Now().AddDate(0, 0, -days)
		criteria.Since = since
	}

	seqNums, err := c.conn.Search(criteria)
	if err != nil {
		log.Printf("Primary search failed: %v", err)
		// Fallback to all
		criteria = imap.NewSearchCriteria()
		seqNums, err = c.conn.Search(criteria)
		if err != nil {
			return nil, fmt.Errorf("search error: %w", err)
		}
	}

	if len(seqNums) == 0 {
		return []MailItem{}, nil
	}

	// Apply limit
	if limit > 0 && len(seqNums) > limit {
		seqNums = seqNums[len(seqNums)-limit:]
	}

	// Reverse so newest is first
	for i, j := 0, len(seqNums)-1; i < j; i, j = i+1, j-1 {
		seqNums[i], seqNums[j] = seqNums[j], seqNums[i]
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(seqNums...)

	messages := make(chan *imap.Message, len(seqNums))
	err = c.conn.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchUid}, messages)
	if err != nil {
		return nil, err
	}

	var results []MailItem
	for msg := range messages {
		subj := msg.Envelope.Subject
		
		fromAddr := ""
		if len(msg.Envelope.From) > 0 {
			f := msg.Envelope.From[0]
			if f.Address() != "" {
				fromAddr = f.PersonalName
				if fromAddr == "" {
					fromAddr = f.Address()
				}
			}
		}

		results = append(results, MailItem{
			ID:              fmt.Sprintf("%d", msg.Uid),
			Subject:         decodeRFC2047(subj),
			From:            decodeRFC2047(fromAddr),
			Date:            msg.Envelope.Date.Format(time.RFC1123Z),
			AttachmentCount: 0,     // We'd need BODYSTRUCTURE to get this realistically, kept 0 for speed for now
			HasAttachments:  false,
		})
	}

	return results, nil
}

func decodeRFC2047(s string) string {
	dec := new(mime.WordDecoder)
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		charset = strings.ToLower(charset)
		if charset == "gb2312" || charset == "gbk" || charset == "gb18030" {
			return transform.NewReader(input, simplifiedchinese.GBK.NewDecoder()), nil
		}
		return input, nil
	}
	res, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return res
}

func (c *MailClient) fetchMessage(mailID string) (*mail.Reader, *imap.Message, error) {
	if err := c.ensureConnection(); err != nil {
		return nil, nil, err
	}

	seqset := new(imap.SeqSet)
	// Python client uses pure search sequence number or UID depending. In string form we assume UID.
	var uid uint32
	if _, err := fmt.Sscanf(mailID, "%d", &uid); err != nil {
		return nil, nil, err
	}
	seqset.AddNum(uid)

	section := &imap.BodySectionName{}
	messages := make(chan *imap.Message, 1)

	// Fetch the full message using UID
	err := c.conn.UidFetch(seqset, []imap.FetchItem{section.FetchItem()}, messages)
	if err != nil {
		return nil, nil, err
	}

	msg := <-messages
	if msg == nil {
		return nil, nil, fmt.Errorf("message not found")
	}
	r := msg.GetBody(section)
	if r == nil {
		return nil, nil, fmt.Errorf("message body not found")
	}

	mr, err := mail.CreateReader(r)
	if err != nil {
		return nil, msg, err
	}
	return mr, msg, nil
}

func (c *MailClient) FetchMailDetail(mailID string) (map[string]interface{}, error) {
	mr, msg, err := c.fetchMessage(mailID)
	if err != nil {
		return nil, err
	}
	_ = msg

	var body, htmlBody string
	var attachments []map[string]interface{}
	
	// Fetch raw content: unfortunately mail.CreateReader consumes the stream
	// For actual raw_content we would read all bytes then parse, simplified here.

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Printf("Error reading part: %v", err)
			break
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			b, _ := io.ReadAll(p.Body)
			contentType, _, _ := h.ContentType()
			if strings.HasPrefix(contentType, "text/html") {
				htmlBody = string(b)
			} else if strings.HasPrefix(contentType, "text/plain") {
				body = string(b)
			}
		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			b, _ := io.ReadAll(p.Body)
			attachments = append(attachments, map[string]interface{}{
				"filename":     decodeRFC2047(filename),
				"size":         len(b),
				"content_type": h.Get("Content-Type"),
			})
		}
	}

	// Simple HTML tag stripping for body if body is empty
	if body == "" && htmlBody != "" {
		body = htmlBody // Simplified, real stripping can be added
	}

	return map[string]interface{}{
		"body":        body,
		"html_body":   htmlBody,
		"attachments": attachments,
		"raw_content": "", // left blank for brevity right now
	}, nil
}

func (c *MailClient) FetchAttachments(mailID string) ([]map[string]interface{}, error) {
	detail, err := c.FetchMailDetail(mailID)
	if err != nil {
		return nil, err
	}
	if att, ok := detail["attachments"].([]map[string]interface{}); ok {
		return att, nil
	}
	return []map[string]interface{}{}, nil
}

func (c *MailClient) DownloadAttachments(mailID string, savePath string) ([]string, error) {
	mr, _, err := c.fetchMessage(mailID)
	if err != nil {
		return nil, err
	}

	var downloaded []string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			break
		}

		switch h := p.Header.(type) {
		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			filename = decodeRFC2047(filename)
			
			// Clean filename (simple version)
			safeFilename := strings.ReplaceAll(filename, "/", "_")
			safeFilename = strings.ReplaceAll(safeFilename, "\\", "_")
			
			fpath := filepath.Join(savePath, safeFilename)
			
			// Save
			b, _ := io.ReadAll(p.Body)
			if len(b) > 0 {
				err = os.WriteFile(fpath, b, 0644)
				if err == nil {
					downloaded = append(downloaded, safeFilename)
				}
			}
		}
	}

	return downloaded, nil
}

