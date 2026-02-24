package mail

import (
	"testing"
)

func TestNewMailClient(t *testing.T) {
	mc := NewMailClient("imap.example.com", 993, "user@example.com", "password123", true)

	if mc.server != "imap.example.com" {
		t.Errorf("expected server 'imap.example.com', got '%s'", mc.server)
	}
	if mc.port != 993 {
		t.Errorf("expected port 993, got %d", mc.port)
	}
	if mc.username != "user@example.com" {
		t.Errorf("expected username 'user@example.com', got '%s'", mc.username)
	}
	if mc.password != "password123" {
		t.Errorf("expected password 'password123', got '%s'", mc.password)
	}
	if !mc.useSSL {
		t.Error("expected useSSL to be true")
	}
	if mc.conn != nil {
		t.Error("expected conn to be nil before Connect()")
	}
}

func TestNewMailClient_NoSSL(t *testing.T) {
	mc := NewMailClient("imap.example.com", 143, "user", "pass", false)
	if mc.useSSL {
		t.Error("expected useSSL to be false")
	}
}

func TestDisconnect_NilConn(t *testing.T) {
	mc := NewMailClient("imap.example.com", 993, "user", "pass", true)
	// Should not panic when conn is nil
	mc.Disconnect()
	if mc.conn != nil {
		t.Error("expected conn to remain nil")
	}
}

func TestFetchMailList_NotConnected(t *testing.T) {
	mc := NewMailClient("imap.example.com", 993, "user", "pass", true)
	_, err := mc.FetchMailList(50, 0)
	if err == nil {
		t.Error("expected error when not connected")
	}
	if err.Error() != "not connected" {
		t.Errorf("expected 'not connected', got '%s'", err.Error())
	}
}

func TestFetchMailDetail_NotConnected(t *testing.T) {
	mc := NewMailClient("imap.example.com", 993, "user", "pass", true)
	_, err := mc.FetchMailDetail("123")
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestFetchAttachments_NotConnected(t *testing.T) {
	mc := NewMailClient("imap.example.com", 993, "user", "pass", true)
	_, err := mc.FetchAttachments("123")
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestDownloadAttachments_NotConnected(t *testing.T) {
	mc := NewMailClient("imap.example.com", 993, "user", "pass", true)
	_, err := mc.DownloadAttachments("123", "/tmp")
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestDecodeRFC2047_PlainText(t *testing.T) {
	result := decodeRFC2047("Hello World")
	if result != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", result)
	}
}

func TestDecodeRFC2047_UTF8Encoded(t *testing.T) {
	// RFC 2047 encoded UTF-8 string: "测试"
	encoded := "=?UTF-8?B?5rWL6K+V?="
	result := decodeRFC2047(encoded)
	if result != "测试" {
		t.Errorf("expected '测试', got '%s'", result)
	}
}

func TestDecodeRFC2047_GBKEncoded(t *testing.T) {
	// RFC 2047 encoded GBK string: "你好" in GBK is \xc4\xe3\xba\xc3
	encoded := "=?GBK?B?xOO6ww==?="
	result := decodeRFC2047(encoded)
	if result != "你好" {
		t.Errorf("expected '你好', got '%s'", result)
	}
}

func TestDecodeRFC2047_EmptyString(t *testing.T) {
	result := decodeRFC2047("")
	if result != "" {
		t.Errorf("expected empty string, got '%s'", result)
	}
}

func TestDecodeRFC2047_InvalidEncoding(t *testing.T) {
	// Should return original string when decoding fails
	input := "=?INVALID?X?broken?="
	result := decodeRFC2047(input)
	// Should not panic, returns something
	if result == "" {
		t.Error("did not expect empty string for invalid encoding")
	}
}
