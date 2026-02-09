package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	textTemplate "text/template"

	"gopkg.in/yaml.v3"
)

//go:embed emails/*
var emailTemplates embed.FS

// EmailConfig holds email configuration from config.yaml
type EmailConfig struct {
	Branding struct {
		Name         string `yaml:"name"`
		Tagline      string `yaml:"tagline"`
		Website      string `yaml:"website"`
		DashboardURL string `yaml:"dashboard_url"`
		DocsURL      string `yaml:"docs_url"`
	} `yaml:"branding"`

	Design struct {
		PrimaryColor   string `yaml:"primary_color"`
		PrimaryHover   string `yaml:"primary_hover"`
		TextColor      string `yaml:"text_color"`
		MutedColor     string `yaml:"muted_color"`
		Background     string `yaml:"background"`
		LightBg        string `yaml:"light_bg"`
		WarningBg      string `yaml:"warning_bg"`
		WarningBorder  string `yaml:"warning_border"`
		CodeBg         string `yaml:"code_bg"`
		BorderColor    string `yaml:"border_color"`
	} `yaml:"design"`

	Subjects struct {
		MagicLink       string `yaml:"magic_link"`
		Welcome         string `yaml:"welcome"`
		PasswordReset   string `yaml:"password_reset"`
		AccountVerified string `yaml:"account_verified"`
	} `yaml:"subjects"`

	MagicLink struct {
		Greeting        string `yaml:"greeting"`
		Intro           string `yaml:"intro"`
		ButtonText      string `yaml:"button_text"`
		ExpiryWarning   string `yaml:"expiry_warning"`
		SecurityNote    string `yaml:"security_note"`
		AlternativeText string `yaml:"alternative_text"`
		IgnoreText      string `yaml:"ignore_text"`
	} `yaml:"magic_link"`

	Welcome struct {
		Greeting         string `yaml:"greeting"`
		Intro            string `yaml:"intro"`
		NowYouCan        string `yaml:"now_you_can"`
		ButtonText       string `yaml:"button_text"`
		NextStepsTitle   string `yaml:"next_steps_title"`
		HelpText         string `yaml:"help_text"`
		Features         []Feature `yaml:"features"`
		Steps            []string  `yaml:"steps"`
	} `yaml:"welcome"`
}

// Feature represents a feature block in welcome email
type Feature struct {
	Icon        string `yaml:"icon"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
}

// LoadEmailConfig loads email configuration from embedded config.yaml (always English)
func LoadEmailConfig(language string) (*EmailConfig, error) {
	data, err := emailTemplates.ReadFile("emails/config.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read email config: %w", err)
	}

	var config EmailConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse email config: %w", err)
	}

	return &config, nil
}

// MagicLinkData holds data for magic link email template
type MagicLinkData struct {
	// User data
	Name string

	// Magic link
	MagicLink     string
	ExpiryMinutes int

	// Config-based data (populated from config.yaml)
	BrandName       string
	Tagline         string
	Website         string
	Greeting        string
	Intro           string
	ButtonText      string
	ExpiryWarning   string
	SecurityNote    string
	AlternativeText string
	IgnoreText      string

	// Design colors
	PrimaryColor  string
	PrimaryHover  string
	TextColor     string
	MutedColor    string
	WarningBg     string
	WarningBorder string
	CodeBg        string
	BorderColor   string
}

// WelcomeData holds data for welcome email template
type WelcomeData struct {
	// User data
	Name string

	// Config-based data
	BrandName      string
	Tagline        string
	Website        string
	DashboardUrl   string
	DocsUrl        string
	Greeting       string
	Intro          string
	NowYouCan      string
	ButtonText     string
	NextStepsTitle string
	HelpText       string
	Features       []Feature
	Steps          []string

	// Design colors
	PrimaryColor string
	PrimaryHover string
	TextColor    string
	MutedColor   string
	LightBg      string
	BorderColor  string
}

// RenderMagicLinkHTML renders magic link HTML template (always English)
func RenderMagicLinkHTML(data MagicLinkData, language string) (string, error) {
	tmplData, err := emailTemplates.ReadFile("emails/magic-link.html")
	if err != nil {
		return "", fmt.Errorf("failed to read magic-link.html: %w", err)
	}

	tmpl, err := template.New("magic-link").Parse(string(tmplData))
	if err != nil {
		return "", fmt.Errorf("failed to parse magic-link template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute magic-link template: %w", err)
	}

	return buf.String(), nil
}

// RenderMagicLinkText renders magic link plain text template (always English)
func RenderMagicLinkText(data MagicLinkData, language string) (string, error) {
	tmplData, err := emailTemplates.ReadFile("emails/magic-link.txt")
	if err != nil {
		return "", fmt.Errorf("failed to read magic-link.txt: %w", err)
	}

	tmpl, err := textTemplate.New("magic-link-text").Parse(string(tmplData))
	if err != nil {
		return "", fmt.Errorf("failed to parse magic-link text template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute magic-link text template: %w", err)
	}

	return buf.String(), nil
}

// RenderWelcomeHTML renders welcome HTML template (always English)
func RenderWelcomeHTML(data WelcomeData, language string) (string, error) {
	tmplData, err := emailTemplates.ReadFile("emails/welcome.html")
	if err != nil {
		return "", fmt.Errorf("failed to read welcome.html: %w", err)
	}

	tmpl, err := template.New("welcome").Parse(string(tmplData))
	if err != nil {
		return "", fmt.Errorf("failed to parse welcome template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute welcome template: %w", err)
	}

	return buf.String(), nil
}

// RenderWelcomeText renders welcome plain text template (always English)
func RenderWelcomeText(data WelcomeData, language string) (string, error) {
	tmplData, err := emailTemplates.ReadFile("emails/welcome.txt")
	if err != nil {
		return "", fmt.Errorf("failed to read welcome.txt: %w", err)
	}

	// Add custom function for incrementing index (for numbered list)
	funcMap := textTemplate.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	tmpl, err := textTemplate.New("welcome-text").Funcs(funcMap).Parse(string(tmplData))
	if err != nil {
		return "", fmt.Errorf("failed to parse welcome text template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute welcome text template: %w", err)
	}

	return buf.String(), nil
}
