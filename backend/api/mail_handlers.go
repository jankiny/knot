package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"knot-backend/mail"

	"github.com/go-chi/chi/v5"
)

var mailClient *mail.MailClient

// -- Mail Handlers --

type MailConfig struct {
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseSSL   bool   `json:"use_ssl"`
}

func handleConnectMail(w http.ResponseWriter, r *http.Request) {
	var config MailConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if mailClient != nil {
		mailClient.Disconnect()
	}

	mailClient = mail.NewMailClient(config.Server, config.Port, config.Username, config.Password, config.UseSSL)
	if err := mailClient.Connect(); err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("连接失败: %v", err))
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true, "message": "连接成功"})
}

func handleGetMailList(w http.ResponseWriter, r *http.Request) {
	if mailClient == nil {
		jsonError(w, http.StatusBadRequest, "请先连接邮箱")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	days, _ := strconv.Atoi(r.URL.Query().Get("days"))

	mails, err := mailClient.FetchMailList(limit, days)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true, "data": mails})
}

func handleGetAttachments(w http.ResponseWriter, r *http.Request) {
	if mailClient == nil {
		jsonError(w, http.StatusBadRequest, "请先连接邮箱")
		return
	}

	mailID := chi.URLParam(r, "mail_id")
	attachments, err := mailClient.FetchAttachments(mailID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true, "data": attachments})
}

func handleGetMailDetail(w http.ResponseWriter, r *http.Request) {
	if mailClient == nil {
		jsonError(w, http.StatusBadRequest, "请先连接邮箱")
		return
	}

	mailID := chi.URLParam(r, "mail_id")
	detail, err := mailClient.FetchMailDetail(mailID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true, "data": detail})
}
