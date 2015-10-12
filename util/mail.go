package util

import (
	"encoding/base64"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"
)

type MailT struct {
	Addr  string // smtp.163.com:25
	User  string // robot@163.com
	Pass  string // 12345
	From  string // 发件人
	To    string // 收件人, 以逗号隔开的多个
	Title string // 邮件标题
	Body  string // 邮件正文
	Type  string // 内容类型，纯文本plain或网页html
}

// 发送邮件
func SendMail(conf *MailT) error {
	var (
		contentType string
		vs          string
		message     string
		toaddr      mail.Address
	)
	encode := base64.NewEncoding("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/")
	host := strings.Split(conf.Addr, ":")
	auth := smtp.PlainAuth("", conf.User, conf.Pass, host[0])
	if conf.Type == "html" {
		contentType = "text/html; charset=UTF-8"
	} else {
		contentType = "text/plain; charset=UTF-8"
	}
	from := mail.Address{"CmsTop Monitor", conf.From}
	tolist := strings.Split(conf.To, ",")
	to := make([]string, 0)
	for i, addr := range tolist {
		addr = strings.TrimSpace(addr)
		tolist[i] = addr
		toaddr = mail.Address{"", tolist[i]}
		to = append(to, toaddr.String())
	}

	header := make(mail.Header)
	header["From"] = []string{from.String()}
	header["To"] = to
	header["Subject"] = []string{conf.Title}
	header["MIME-Version"] = []string{"1.0"}
	header["Content-Type"] = []string{contentType}
	header["Content-Transfer-Encoding"] = []string{"base64"}

	for k, v := range header {
		vs = strings.Join(v, ", ")
		message += fmt.Sprintf("%s: %s\r\n", k, vs)
	}
	message += "\r\n" + encode.EncodeToString([]byte(conf.Body))

	err := smtp.SendMail(
		conf.Addr,
		auth,
		from.Address,
		tolist,
		[]byte(message),
	)
	return err
}
