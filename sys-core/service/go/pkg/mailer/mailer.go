package mailer

import (
	"context"
	"fmt"
	"github.com/matcornic/hermes/v2"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"go.amplifyedge.org/sys-share-v2/sys-core/service/logging"
	"gopkg.in/gomail.v2"

	coreRpc "go.amplifyedge.org/sys-share-v2/sys-core/service/go/rpc/v2"
	service "go.amplifyedge.org/sys-v2/sys-core/service/go"
)

type MailSvc struct {
	senderName string
	senderMail string
	client     *sendgrid.Client
	dialer     *gomail.Dialer
	// smtpCfg    *service.SmtpConfig
	logger logging.Logger
	hp     hermes.Product
	*coreRpc.UnimplementedEmailServiceServer
}

func NewMailSvc(mcfg *service.MailConfig, l logging.Logger) *MailSvc {
	mailSvc := &MailSvc{
		senderName: mcfg.SenderName,
		senderMail: mcfg.SenderMail,
		logger:     l,
		hp: hermes.Product{
			Name:        mcfg.ProductName,
			Logo:        mcfg.LogoUrl,
			Copyright:   mcfg.Copyright,
			TroubleText: mcfg.TroubleContact,
		},
	}
	if &mcfg.Smtp != nil {
		mailSvc.dialer = gomail.NewDialer(
			mcfg.Smtp.Host,
			mcfg.Smtp.Port,
			mcfg.Smtp.Email,
			mcfg.Smtp.Password,
		)
		// mailSvc.smtpCfg = &mcfg.Smtp
	}
	if &mcfg.Sendgrid != nil && mcfg.Sendgrid.ApiKey != "" {
		mailSvc.client = sendgrid.NewSendClient(mcfg.Sendgrid.ApiKey)
	}

	return mailSvc
}

func (m *MailSvc) GetHermesProduct() hermes.Product {
	return m.hp
}

func (m *MailSvc) SendMail(ctx context.Context, in *coreRpc.EmailRequest) (*coreRpc.EmailResponse, error) {
	if m.dialer != nil {
		for name, address := range in.Recipients {
			msg := gomail.NewMessage()
			msg.SetAddressHeader("To", address, name)
			msg.SetAddressHeader("From", m.senderMail, m.senderName)
			msg.SetHeader("Subject", in.Subject)
			msg.SetBody("text/html", string(in.Content))
			if err := m.dialer.DialAndSend(msg); err != nil {
				return &coreRpc.EmailResponse{
					Success:        false,
					ErrMessage:     err.Error(),
					SuccessMessage: "",
				}, err
			}
		}
		return &coreRpc.EmailResponse{
			Success:        false,
			ErrMessage:     "",
			SuccessMessage: "Successfully sent all emails",
		}, nil
	}
	if m.client != nil {
		sender := mail.NewEmail(m.senderName, m.senderMail)
		content := string(in.Content)
		for name, address := range in.Recipients {
			msg := mail.NewSingleEmail(sender, in.Subject, mail.NewEmail(name, address), content, content)
			resp, err := m.client.Send(msg)
			if err != nil {
				return &coreRpc.EmailResponse{
					Success:        false,
					ErrMessage:     err.Error(),
					SuccessMessage: "",
				}, err
			}
			m.logger.Debugf("Email response: %s", resp.Body)
		}
		return &coreRpc.EmailResponse{
			Success:        true,
			ErrMessage:     "",
			SuccessMessage: "Successfully sent all emails",
		}, nil
	}
	return nil, fmt.Errorf("error: all alternative email sender is nil")
}
