package services

import (
	"context"
	"fmt"
	"time"

	"github.com/khabaroff/obsidian-webhooks-selfhosted/src/templates"
	"github.com/mailgun/mailgun-go/v4"
)

// EmailService handles transactional email sending via Mailgun
type EmailService struct {
	mg        *mailgun.MailgunImpl
	fromEmail string
	fromName  string
	domain    string
}

// NewEmailService creates a new email service with Mailgun configuration
func NewEmailService(domain, apiKey, fromEmail, fromName string) *EmailService {
	mg := mailgun.NewMailgun(domain, apiKey)
	mg.SetAPIBase(mailgun.APIBaseEU) // Use EU endpoint for GDPR compliance

	return &EmailService{
		mg:        mg,
		fromEmail: fromEmail,
		fromName:  fromName,
		domain:    domain,
	}
}

// getDefaultEmailConfig returns default email configuration as fallback
func getDefaultEmailConfig() *templates.EmailConfig {
	return &templates.EmailConfig{
		Branding: struct {
			Name         string `yaml:"name"`
			Tagline      string `yaml:"tagline"`
			Website      string `yaml:"website"`
			DashboardURL string `yaml:"dashboard_url"`
			DocsURL      string `yaml:"docs_url"`
		}{
			Name:         "Khabaroff Studio: Obsidian Webhooks",
			Tagline:      "Webhook delivery to Obsidian",
			Website:      "https://obsidian-webhooks.khabaroff.studio",
			DashboardURL: "https://obsidian-webhooks.khabaroff.studio/dashboard",
			DocsURL:      "https://obsidian-webhooks.khabaroff.studio/guides/",
		},
		Design: struct {
			PrimaryColor  string `yaml:"primary_color"`
			PrimaryHover  string `yaml:"primary_hover"`
			TextColor     string `yaml:"text_color"`
			MutedColor    string `yaml:"muted_color"`
			Background    string `yaml:"background"`
			LightBg       string `yaml:"light_bg"`
			WarningBg     string `yaml:"warning_bg"`
			WarningBorder string `yaml:"warning_border"`
			CodeBg        string `yaml:"code_bg"`
			BorderColor   string `yaml:"border_color"`
		}{
			PrimaryColor:  "#7C3AED",
			PrimaryHover:  "#7C3AED",
			TextColor:     "#0a0a0a",
			MutedColor:    "#777777",
			Background:    "#ffffff",
			LightBg:       "#f5f5f5",
			WarningBg:     "#f5f5f5",
			WarningBorder: "#7C3AED",
			CodeBg:        "#f5f5f5",
			BorderColor:   "#e5e5e5",
		},
		Subjects: struct {
			MagicLink       string `yaml:"magic_link"`
			Welcome         string `yaml:"welcome"`
			PasswordReset   string `yaml:"password_reset"`
			AccountVerified string `yaml:"account_verified"`
		}{
			MagicLink:       "Your login link — Obsidian Webhooks",
			Welcome:         "Your API keys are ready — Obsidian Webhooks",
			PasswordReset:   "Reset your password",
			AccountVerified: "Your account is verified",
		},
	}
}

// SendMagicLinkEmail sends a magic link authentication email (always English)
func (s *EmailService) SendMagicLinkEmail(ctx context.Context, toEmail, toName, magicLink string, expiryMinutes int, language string) error {
	config, err := templates.LoadEmailConfig("en")
	if err != nil {
		config = getDefaultEmailConfig()
	}

	subject := config.Subjects.MagicLink

	displayName := toName
	if displayName == "" {
		displayName = "there"
	}

	data := templates.MagicLinkData{
		Name:            displayName,
		MagicLink:       magicLink,
		ExpiryMinutes:   expiryMinutes,
		BrandName:       config.Branding.Name,
		Tagline:         config.Branding.Tagline,
		Website:         config.Branding.Website,
		Greeting:        fmt.Sprintf("Hi %s,", displayName),
		Intro:           config.MagicLink.Intro,
		ButtonText:      config.MagicLink.ButtonText,
		ExpiryWarning:   fmt.Sprintf(config.MagicLink.ExpiryWarning, expiryMinutes),
		SecurityNote:    config.MagicLink.SecurityNote,
		AlternativeText: config.MagicLink.AlternativeText,
		IgnoreText:      config.MagicLink.IgnoreText,
		PrimaryColor:    config.Design.PrimaryColor,
		PrimaryHover:    config.Design.PrimaryHover,
		TextColor:       config.Design.TextColor,
		MutedColor:      config.Design.MutedColor,
		WarningBg:       config.Design.WarningBg,
		WarningBorder:   config.Design.WarningBorder,
		CodeBg:          config.Design.CodeBg,
		BorderColor:     config.Design.BorderColor,
	}

	// Render templates
	htmlBody, err := templates.RenderMagicLinkHTML(data, language)
	if err != nil {
		// Fallback to old hardcoded template
		htmlBody = s.getMagicLinkHTMLTemplate(toName, magicLink, expiryMinutes)
	}

	textBody, err := templates.RenderMagicLinkText(data, language)
	if err != nil {
		// Fallback to old hardcoded template
		textBody = s.getMagicLinkPlainTextTemplate(toName, magicLink, expiryMinutes)
	}

	message := s.mg.NewMessage(
		fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		subject,
		textBody,
		toEmail,
	)
	message.SetHtml(htmlBody)

	// Set timeout for sending
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	_, _, err = s.mg.Send(ctxWithTimeout, message)
	if err != nil {
		return fmt.Errorf("failed to send magic link email to %s: %w", toEmail, err)
	}

	return nil
}

// SendWelcomeEmail sends a welcome email to new users (always English)
func (s *EmailService) SendWelcomeEmail(ctx context.Context, toEmail, toName, language string) error {
	config, err := templates.LoadEmailConfig("en")
	if err != nil {
		config = getDefaultEmailConfig()
	}

	subject := config.Subjects.Welcome

	displayName := toName
	if displayName == "" {
		displayName = "there"
	}

	data := templates.WelcomeData{
		Name:           displayName,
		BrandName:      config.Branding.Name,
		Tagline:        config.Branding.Tagline,
		Website:        config.Branding.Website,
		DashboardUrl:   config.Branding.DashboardURL,
		DocsUrl:        config.Branding.DocsURL,
		Greeting:       fmt.Sprintf("Hi %s,", displayName),
		Intro:          config.Welcome.Intro,
		NowYouCan:      config.Welcome.NowYouCan,
		ButtonText:     config.Welcome.ButtonText,
		NextStepsTitle: config.Welcome.NextStepsTitle,
		HelpText:       config.Welcome.HelpText,
		Features:       config.Welcome.Features,
		Steps:          config.Welcome.Steps,
		PrimaryColor:   config.Design.PrimaryColor,
		PrimaryHover:   config.Design.PrimaryHover,
		TextColor:      config.Design.TextColor,
		MutedColor:     config.Design.MutedColor,
		LightBg:        config.Design.LightBg,
		BorderColor:    config.Design.BorderColor,
	}

	// Render templates
	htmlBody, err := templates.RenderWelcomeHTML(data, language)
	if err != nil {
		// Fallback to old hardcoded template
		htmlBody = s.getWelcomeHTMLTemplate(toName)
	}

	textBody, err := templates.RenderWelcomeText(data, language)
	if err != nil {
		// Fallback to old hardcoded template
		textBody = s.getWelcomePlainTextTemplate(toName)
	}

	message := s.mg.NewMessage(
		fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		subject,
		textBody,
		toEmail,
	)
	message.SetHtml(htmlBody)

	// Set timeout for sending
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	_, _, err = s.mg.Send(ctxWithTimeout, message)
	if err != nil {
		return fmt.Errorf("failed to send welcome email to %s: %w", toEmail, err)
	}

	return nil
}

// getMagicLinkHTMLTemplate returns the HTML template for magic link email
func (s *EmailService) getMagicLinkHTMLTemplate(name, magicLink string, expiryMinutes int) string {
	displayName := name
	if displayName == "" {
		displayName = "there"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sign in — Obsidian Webhooks</title>
</head>
<body style="margin:0;padding:0;background:#fff;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#0a0a0a;line-height:1.6;">
    <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
        <tr><td style="padding:40px 24px 24px;border-bottom:2px solid #e5e5e5;">
            <span style="font-size:11px;font-weight:700;letter-spacing:2px;text-transform:uppercase;">OBSIDIAN WEBHOOKS</span>
        </td></tr>
        <tr><td style="padding:32px 24px;">
            <h2 style="margin:0 0 16px;font-size:20px;font-weight:700;">Hi %s,</h2>
            <p style="margin:0 0 24px;font-size:15px;color:#444;">Click the button below to sign in to your dashboard.</p>
            <table role="presentation" cellpadding="0" cellspacing="0"><tr>
                <td style="background:#7C3AED;padding:14px 32px;">
                    <a href="%s" style="color:#fff;text-decoration:none;font-size:14px;font-weight:600;">Sign in</a>
                </td>
            </tr></table>
            <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin:24px 0;"><tr>
                <td style="background:#f5f5f5;padding:14px 16px;border-left:3px solid #7C3AED;">
                    <span style="font-size:13px;font-weight:600;">This link expires in %d minutes.</span><br>
                    <span style="font-size:13px;color:#777;">Single use only. Do not share.</span>
                </td>
            </tr></table>
            <p style="margin:0 0 8px;font-size:13px;color:#777;">Or copy this link into your browser:</p>
            <p style="margin:0 0 24px;font-size:12px;font-family:monospace;color:#777;word-break:break-all;background:#f5f5f5;padding:8px 12px;">%s</p>
            <p style="margin:0;font-size:13px;color:#777;">Didn't request this? Ignore this email.</p>
        </td></tr>
        <tr><td style="padding:24px;border-top:1px solid #e5e5e5;">
            <p style="margin:0 0 4px;font-size:12px;color:#777;">Khabaroff Studio: Obsidian Webhooks — Webhook delivery to Obsidian</p>
            <a href="https://obsidian-webhooks.khabaroff.studio" style="font-size:12px;color:#777;">obsidian-webhooks.khabaroff.studio</a>
        </td></tr>
    </table>
</body>
</html>`, displayName, magicLink, expiryMinutes, magicLink)
}

// getMagicLinkPlainTextTemplate returns the plain text template for magic link email
func (s *EmailService) getMagicLinkPlainTextTemplate(name, magicLink string, expiryMinutes int) string {
	displayName := name
	if displayName == "" {
		displayName = "there"
	}

	return fmt.Sprintf(`Hi %s,

Click the link below to sign in to your dashboard.

%s

This link expires in %d minutes. Single use only.

Didn't request this? Ignore this email.

—
Khabaroff Studio: Obsidian Webhooks
Webhook delivery to Obsidian
https://obsidian-webhooks.khabaroff.studio`, displayName, magicLink, expiryMinutes)
}

// getWelcomeHTMLTemplate returns the HTML template for welcome email
func (s *EmailService) getWelcomeHTMLTemplate(name string) string {
	displayName := name
	if displayName == "" {
		displayName = "there"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome — Obsidian Webhooks</title>
</head>
<body style="margin:0;padding:0;background:#fff;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#0a0a0a;line-height:1.6;">
    <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;">
        <tr><td style="padding:40px 24px 24px;border-bottom:2px solid #e5e5e5;">
            <span style="font-size:11px;font-weight:700;letter-spacing:2px;text-transform:uppercase;">OBSIDIAN WEBHOOKS</span>
        </td></tr>
        <tr><td style="padding:32px 24px;">
            <h2 style="margin:0 0 16px;font-size:20px;font-weight:700;">Hi %s,</h2>
            <p style="margin:0 0 24px;font-size:15px;color:#444;">Your account is ready.</p>
            <p style="margin:0 0 12px;font-size:14px;font-weight:600;">What you can do:</p>
            <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:8px;"><tr>
                <td style="background:#f5f5f5;padding:14px 16px;border-left:3px solid #7C3AED;">
                    <span style="font-size:14px;font-weight:600;">Send webhooks to Obsidian</span><br>
                    <span style="font-size:13px;color:#444;">Connect n8n, Make, scripts, or any HTTP client to your vault</span>
                </td>
            </tr></table>
            <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:8px;"><tr>
                <td style="background:#f5f5f5;padding:14px 16px;border-left:3px solid #7C3AED;">
                    <span style="font-size:14px;font-weight:600;">Automatic note creation</span><br>
                    <span style="font-size:13px;color:#444;">Every webhook becomes a note in the right folder</span>
                </td>
            </tr></table>
            <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:8px;"><tr>
                <td style="background:#f5f5f5;padding:14px 16px;border-left:3px solid #7C3AED;">
                    <span style="font-size:14px;font-weight:600;">Real-time and offline delivery</span><br>
                    <span style="font-size:13px;color:#444;">Real-time when online, queued when offline (up to 30 days)</span>
                </td>
            </tr></table>
            <table role="presentation" cellpadding="0" cellspacing="0" style="margin:24px 0;"><tr>
                <td style="background:#7C3AED;padding:14px 32px;">
                    <a href="https://obsidian-webhooks.khabaroff.studio/dashboard" style="color:#fff;text-decoration:none;font-size:14px;font-weight:600;">Open Dashboard</a>
                </td>
            </tr></table>
            <p style="margin:0 0 8px;font-size:14px;font-weight:600;">Quick start:</p>
            <p style="margin:0;font-size:14px;color:#444;">1. Download the Obsidian plugin from your dashboard<br>2. Paste your client_key in plugin settings<br>3. Send a test webhook — note appears in 1-3 seconds</p>
            <p style="margin:24px 0 0;font-size:13px;color:#777;">Questions? Check the <a href="https://github.com/khabaroff-studio/obsidian-webhooks-server" style="color:#0a0a0a;font-weight:600;">docs on GitHub</a> or reply to this email.</p>
        </td></tr>
        <tr><td style="padding:24px;border-top:1px solid #e5e5e5;">
            <p style="margin:0 0 4px;font-size:12px;color:#777;">Khabaroff Studio: Obsidian Webhooks — Webhook delivery to Obsidian</p>
            <a href="https://obsidian-webhooks.khabaroff.studio" style="font-size:12px;color:#777;">obsidian-webhooks.khabaroff.studio</a>
        </td></tr>
    </table>
</body>
</html>`, displayName)
}

// getWelcomePlainTextTemplate returns the plain text template for welcome email
func (s *EmailService) getWelcomePlainTextTemplate(name string) string {
	displayName := name
	if displayName == "" {
		displayName = "there"
	}

	return fmt.Sprintf(`Hi %s,

Your account is ready.

What you can do:

— Send webhooks to Obsidian
  Connect n8n, Make, scripts, or any HTTP client to your vault

— Automatic note creation
  Every webhook becomes a note in the right folder

— Real-time and offline delivery
  Real-time when online, queued when offline (up to 30 days)

Quick start:
1. Download the Obsidian plugin from your dashboard
2. Paste your client_key in plugin settings
3. Send a test webhook — note appears in 1-3 seconds

Open your dashboard: https://obsidian-webhooks.khabaroff.studio/dashboard

Questions? Check the docs on GitHub or reply to this email.

—
Khabaroff Studio: Obsidian Webhooks
Webhook delivery to Obsidian
https://obsidian-webhooks.khabaroff.studio`, displayName)
}
