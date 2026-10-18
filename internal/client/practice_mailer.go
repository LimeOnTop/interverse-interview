package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	authpb "github.com/LimeOnTop/interverse-contracts/auth/gen"
	"github.com/LimeOnTop/interverse-interview/internal/service"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"html"
	"log"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

type PracticeSMTPConfig struct {
	Host, Port, Username, Password, From, FrontendURL, AuthURL string
	DevMode                                                    bool
}
type PracticeMailer struct {
	cfg  PracticeSMTPConfig
	auth authpb.AuthServiceClient
	from *mail.Address
	conn *grpc.ClientConn
}

func NewPracticeMailer(cfg PracticeSMTPConfig) (*PracticeMailer, error) {
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = "https://inter-verse.ru"
	}
	if cfg.Port == "" {
		cfg.Port = "587"
	}
	from, e := mail.ParseAddress(cfg.From)
	if e != nil {
		return nil, e
	}
	conn, e := grpc.NewClient(cfg.AuthURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if e != nil {
		return nil, e
	}
	return &PracticeMailer{cfg: cfg, auth: authpb.NewAuthServiceClient(conn), from: from, conn: conn}, nil
}
func (m *PracticeMailer) Close() { m.conn.Close() }
func (m *PracticeMailer) Send(ctx context.Context, p service.PracticeMail) (deliveryErr error) {
	dataStarted := false
	defer func() {
		if deliveryErr == nil {
			return
		}
		var smtpErr *textproto.Error
		var networkErr net.Error
		retryable := false
		uncertain := false
		if errors.As(deliveryErr, &smtpErr) {
			retryable = smtpErr.Code >= 400 && smtpErr.Code < 500
		} else if dataStarted {
			uncertain = true
		} else {
			retryable = errors.As(deliveryErr, &networkErr) || errors.Is(deliveryErr, context.DeadlineExceeded) || status.Code(deliveryErr) == codes.Unavailable || status.Code(deliveryErr) == codes.DeadlineExceeded
		}
		deliveryErr = &service.PracticeDeliveryError{Err: deliveryErr, Retryable: retryable, Uncertain: uncertain}
	}()
	if m.cfg.Host == "" {
		if !m.cfg.DevMode {
			return fmt.Errorf("SMTP unavailable")
		}
		log.Printf("practice reminder dev delivery: user_id=%s", p.UserID)
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	user, e := m.auth.GetUser(ctx, &authpb.GetUserRequest{Id: p.UserID})
	if e != nil {
		return e
	}
	if user.GetUser() == nil || !user.GetResponse().GetSuccess() {
		return fmt.Errorf("recipient unavailable")
	}
	to := user.GetUser().GetEmail()
	if _, e = mail.ParseAddress(to); e != nil {
		return e
	}
	if strings.HasSuffix(strings.ToLower(to), ".test") {
		return fmt.Errorf("test address not deliverable")
	}
	loc, e := time.LoadLocation(p.Timezone)
	if e != nil {
		return e
	}
	date := p.ScheduledAt.In(loc).Format("02.01.2006 в 15:04")
	link := strings.TrimRight(m.cfg.FrontendURL, "/") + p.Path
	body := fmt.Sprintf("Практика в InterVerse: %s\n\nВы запланировали её на %s (%s).\nОткрыть практику: %s\n\nСейчас неудобно? По этой же ссылке можно перенести время или убрать напоминание. Вы получили одно письмо, потому что включили его в настройках практики.\n", p.Title, date, p.Timezone, link)
	htmlBody := fmt.Sprintf(`<html><body style="font-family:Arial,sans-serif;background:#f5f2ea;color:#26332b;padding:24px"><div style="max-width:480px;margin:auto;background:#fffdf8;padding:28px;border-radius:12px"><p>INTERVERSE</p><h2>Время для практики</h2><p>%s</p><p>%s (%s)</p><p><a href="%s" style="display:inline-block;padding:12px 18px;background:#315d46;color:white;text-decoration:none;border-radius:8px">Открыть практику</a></p><p style="font-size:12px">По этой же ссылке можно перенести время или убрать напоминание. Вы включили одно письмо для этой практики.</p></div></body></html>`, html.EscapeString(p.Title), html.EscapeString(date), html.EscapeString(p.Timezone), html.EscapeString(link))
	boundary := uuid.NewString()
	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@inter-verse.ru>\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n", m.from.String(), (&mail.Address{Address: to}).String(), mime.QEncoding.Encode("utf-8", "Ваша практика в InterVerse"), time.Now().Format(time.RFC1123Z), boundary, boundary)
	for _, part := range []struct{ kind, body string }{{"text/plain", body}, {"text/html", htmlBody}} {
		fmt.Fprintf(&msg, "--%s\r\nContent-Type: %s; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary, part.kind)
		w := quotedprintable.NewWriter(&msg)
		w.Write([]byte(part.body))
		w.Close()
		msg.WriteString("\r\n")
	}
	fmt.Fprintf(&msg, "--%s--\r\n", boundary)
	tlsCfg := &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}
	dialer := net.Dialer{}
	conn, e := dialer.DialContext(ctx, "tcp", net.JoinHostPort(m.cfg.Host, m.cfg.Port))
	if e != nil {
		return e
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}
	if m.cfg.Port == "465" {
		conn = tls.Client(conn, tlsCfg)
	}
	c, e := smtp.NewClient(conn, m.cfg.Host)
	if e != nil {
		return e
	}
	defer c.Close()
	if m.cfg.Port != "465" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("STARTTLS unavailable")
		}
		if e = c.StartTLS(tlsCfg); e != nil {
			return e
		}
	}
	if m.cfg.Username != "" {
		if e = c.Auth(smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)); e != nil {
			return e
		}
	}
	if e = c.Mail(m.from.Address); e != nil {
		return e
	}
	if e = c.Rcpt(to); e != nil {
		return e
	}
	w, e := c.Data()
	if e != nil {
		return e
	}
	dataStarted = true
	if _, e = w.Write(msg.Bytes()); e != nil {
		return e
	}
	if e = w.Close(); e != nil {
		return e
	}
	_ = c.Quit()
	return nil
}
