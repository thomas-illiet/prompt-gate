package setupguide

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	CompatibilityOpenAI    = "openai"
	CompatibilityAnthropic = "anthropic"
	CompatibilityBoth      = "both"
	ModelModeSingle        = "single"
	ModelModeAll           = "all"
	ModelModeNone          = "none"
)

var (
	ErrNotFound       = errors.New("setup guide not found")
	identifierPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	iconPattern       = regexp.MustCompile(`^mdi-[a-z0-9-]+$`)
	tagPattern        = regexp.MustCompile(`\{\{\s*([#/]?)([a-zA-Z][a-zA-Z0-9]*)\s*\}\}`)
	allowedVariables  = map[string]bool{
		"token": true, "baseUrl": true, "openaiBaseUrl": true, "anthropicBaseUrl": true,
		"model": true, "models": true, "providerName": true, "providerDisplayName": true,
	}
)

type StringList []string

func (s StringList) Value() (driver.Value, error) { return json.Marshal(s) }
func (s *StringList) Scan(value any) error {
	if value == nil {
		*s = StringList{}
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("scan string list: %T", value)
	}
	return json.Unmarshal(raw, s)
}

type SetupGuide struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Identifier    string     `gorm:"uniqueIndex;not null" json:"identifier"`
	Title         string     `gorm:"not null" json:"title"`
	Subtitle      string     `gorm:"not null" json:"subtitle"`
	Icon          string     `gorm:"not null" json:"icon"`
	Compatibility string     `gorm:"not null;index" json:"compatibility"`
	ModelMode     string     `gorm:"not null" json:"modelMode"`
	FilePaths     StringList `gorm:"type:jsonb;not null" json:"filePaths"`
	Template      string     `gorm:"type:text;not null" json:"template"`
	Enabled       bool       `gorm:"not null;index" json:"enabled"`
	Position      int        `gorm:"not null;index" json:"position"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func (g *SetupGuide) BeforeCreate(_ *gorm.DB) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	return nil
}

type Input struct {
	Identifier    string   `json:"identifier"`
	Title         string   `json:"title"`
	Subtitle      string   `json:"subtitle"`
	Icon          string   `json:"icon"`
	Compatibility string   `json:"compatibility"`
	ModelMode     string   `json:"modelMode"`
	FilePaths     []string `json:"filePaths"`
	Template      string   `json:"template"`
	Enabled       bool     `json:"enabled"`
	Position      int      `json:"position"`
}

type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) AutoMigrate(ctx context.Context) error {
	if err := s.db.WithContext(ctx).AutoMigrate(&SetupGuide{}); err != nil {
		return err
	}
	return s.seedDefaults(ctx)
}

func ValidateTemplate(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("template is required")
	}
	stack := []string{}
	for _, match := range tagPattern.FindAllStringSubmatch(value, -1) {
		kind, name := match[1], match[2]
		if !allowedVariables[name] {
			return fmt.Errorf("unknown template variable %q", name)
		}
		if kind == "#" {
			if name != "models" {
				return fmt.Errorf("sections are only supported for models")
			}
			stack = append(stack, name)
		} else if kind == "/" {
			if len(stack) == 0 || stack[len(stack)-1] != name {
				return fmt.Errorf("unmatched closing section %q", name)
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) > 0 {
		return fmt.Errorf("unclosed section %q", stack[len(stack)-1])
	}
	cleaned := tagPattern.ReplaceAllString(value, "")
	if strings.Contains(cleaned, "{{") || strings.Contains(cleaned, "}}") {
		return errors.New("malformed template tag")
	}
	return nil
}

func validateInput(in Input) error {
	in.Identifier = strings.TrimSpace(in.Identifier)
	if !identifierPattern.MatchString(in.Identifier) {
		return errors.New("identifier must be lowercase kebab-case")
	}
	if strings.TrimSpace(in.Title) == "" {
		return errors.New("title is required")
	}
	if !iconPattern.MatchString(strings.TrimSpace(in.Icon)) {
		return errors.New("icon must be an mdi-* identifier")
	}
	if in.Compatibility != CompatibilityOpenAI && in.Compatibility != CompatibilityAnthropic && in.Compatibility != CompatibilityBoth {
		return errors.New("invalid compatibility")
	}
	if in.ModelMode != ModelModeSingle && in.ModelMode != ModelModeAll && in.ModelMode != ModelModeNone {
		return errors.New("invalid model mode")
	}
	if in.Position < 0 {
		return errors.New("position must be non-negative")
	}
	return ValidateTemplate(in.Template)
}

func fromInput(in Input) SetupGuide {
	paths := make(StringList, 0, len(in.FilePaths))
	for _, p := range in.FilePaths {
		if p = strings.TrimSpace(p); p != "" {
			paths = append(paths, p)
		}
	}
	return SetupGuide{Identifier: strings.TrimSpace(in.Identifier), Title: strings.TrimSpace(in.Title), Subtitle: strings.TrimSpace(in.Subtitle), Icon: strings.TrimSpace(in.Icon), Compatibility: in.Compatibility, ModelMode: in.ModelMode, FilePaths: paths, Template: in.Template, Enabled: in.Enabled, Position: in.Position}
}

func (s *Service) List(ctx context.Context, activeOnly bool) ([]SetupGuide, error) {
	q := s.db.WithContext(ctx).Order("position ASC, title ASC")
	if activeOnly {
		q = q.Where("enabled = ?", true)
	}
	var out []SetupGuide
	return out, q.Find(&out).Error
}
func (s *Service) Get(ctx context.Context, id uuid.UUID) (SetupGuide, error) {
	var out SetupGuide
	err := s.db.WithContext(ctx).First(&out, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return out, ErrNotFound
	}
	return out, err
}
func (s *Service) Create(ctx context.Context, in Input) (SetupGuide, error) {
	if err := validateInput(in); err != nil {
		return SetupGuide{}, err
	}
	out := fromInput(in)
	err := s.db.WithContext(ctx).Create(&out).Error
	return out, err
}
func (s *Service) Update(ctx context.Context, id uuid.UUID, in Input) (SetupGuide, error) {
	if err := validateInput(in); err != nil {
		return SetupGuide{}, err
	}
	out := fromInput(in)
	out.ID = id
	result := s.db.WithContext(ctx).Model(&SetupGuide{}).Where("id = ?", id).Updates(map[string]any{"identifier": out.Identifier, "title": out.Title, "subtitle": out.Subtitle, "icon": out.Icon, "compatibility": out.Compatibility, "model_mode": out.ModelMode, "file_paths": out.FilePaths, "template": out.Template, "enabled": out.Enabled, "position": out.Position})
	if result.Error != nil {
		return SetupGuide{}, result.Error
	}
	if result.RowsAffected == 0 {
		return SetupGuide{}, ErrNotFound
	}
	return s.Get(ctx, id)
}
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Delete(&SetupGuide{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Service) Reorder(ctx context.Context, ids []uuid.UUID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&SetupGuide{}).Count(&count).Error; err != nil {
			return err
		}
		if int64(len(ids)) != count {
			return errors.New("reorder must include every setup guide")
		}
		seen := map[uuid.UUID]bool{}
		for position, id := range ids {
			if seen[id] {
				return errors.New("duplicate setup guide id")
			}
			seen[id] = true
			result := tx.Model(&SetupGuide{}).Where("id = ?", id).Update("position", position)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrNotFound
			}
		}
		return nil
	})
}

func (s *Service) seedDefaults(ctx context.Context) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&SetupGuide{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	defaults := defaultGuides()
	return s.db.WithContext(ctx).Create(&defaults).Error
}

func defaultGuides() []SetupGuide {
	token := "<PROMPTGATE_TOKEN>"
	items := []SetupGuide{
		{Identifier: "curl", Title: "curl", Subtitle: "OpenAI-compatible chat completions request.", Icon: "mdi-console-line", Compatibility: "openai", ModelMode: "single", Template: "curl {{baseUrl}}/chat/completions \\\n  -H \"Authorization: Bearer {{token}}\" \\\n  -H \"Content-Type: application/json\" \\\n  -d '{\"model\":\"{{model}}\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello from PromptGate\"}],\"stream\":true}'"},
		{Identifier: "python", Title: "Python", Subtitle: "OpenAI Python SDK configuration.", Icon: "mdi-language-python", Compatibility: "openai", ModelMode: "single", Template: "from openai import OpenAI\n\nclient = OpenAI(api_key=\"{{token}}\", base_url=\"{{baseUrl}}\")\nresponse = client.chat.completions.create(model=\"{{model}}\", messages=[{\"role\": \"user\", \"content\": \"Hello from PromptGate\"}])"},
		{Identifier: "go", Title: "Go", Subtitle: "OpenAI Go SDK configuration.", Icon: "mdi-language-go", Compatibility: "openai", ModelMode: "single", Template: "client := openai.NewClient(option.WithAPIKey(\"{{token}}\"), option.WithBaseURL(\"{{baseUrl}}\"))\n// Use model {{model}}"},
		{Identifier: "java", Title: "Java", Subtitle: "OpenAI Java SDK configuration.", Icon: "mdi-language-java", Compatibility: "openai", ModelMode: "single", Template: "OpenAIClient client = OpenAIOkHttpClient.builder()\n    .apiKey(\"{{token}}\")\n    .baseUrl(\"{{baseUrl}}\")\n    .build();\n// Use model {{model}}"},
		{Identifier: "aspnet", Title: "ASP.NET", Subtitle: "OpenAI-compatible .NET configuration.", Icon: "mdi-dot-net", Compatibility: "openai", ModelMode: "single", Template: "builder.Services.AddOpenAIChatClient(\"{{model}}\", \"{{token}}\", new Uri(\"{{baseUrl}}\"));"},
		{Identifier: "powershell", Title: "PowerShell", Subtitle: "PowerShell chat completions request.", Icon: "mdi-powershell", Compatibility: "openai", ModelMode: "single", Template: "$headers = @{ Authorization = \"Bearer {{token}}\" }\nInvoke-RestMethod -Uri \"{{baseUrl}}/chat/completions\" -Method Post -Headers $headers -Body '{\"model\":\"{{model}}\"}'"},
		{Identifier: "lua", Title: "Lua", Subtitle: "Lua OpenAI-compatible request.", Icon: "mdi-language-lua", Compatibility: "openai", ModelMode: "single", Template: "local endpoint = \"{{baseUrl}}/chat/completions\"\nlocal token = \"{{token}}\"\nlocal model = \"{{model}}\""},
		{Identifier: "cline", Title: "Cline", Subtitle: "Cline OpenAI-compatible provider settings.", Icon: "mdi-robot-outline", Compatibility: "openai", ModelMode: "single", Template: "API Provider: OpenAI Compatible\nBase URL: {{baseUrl}}\nAPI Key: {{token}}\nModel ID: {{model}}"},
		{Identifier: "continue", Title: "Continue", Subtitle: "Continue config with the OpenAI-compatible provider.", Icon: "mdi-infinity", Compatibility: "openai", ModelMode: "all", FilePaths: StringList{"~/.continue/config.yaml", "%USERPROFILE%\\.continue\\config.yaml"}, Template: "name: PromptGate\nversion: 0.0.1\nschema: v1\n\nmodels:\n{{#models}}  - name: \"{{model}}\"\n    provider: openai\n    model: \"{{model}}\"\n    apiKey: \"{{token}}\"\n    apiBase: \"{{baseUrl}}\"\n{{/models}}"},
		{Identifier: "openclaw", Title: "OpenClaw", Subtitle: "OpenClaw JSON5 config with a custom OpenAI-compatible provider.", Icon: "mdi-application-braces-outline", Compatibility: "openai", ModelMode: "all", FilePaths: StringList{"~/.openclaw/openclaw.json"}, Template: "{\n  env: { PROMPTGATE_TOKEN: \"{{token}}\" },\n  models: [\n{{#models}}    { id: \"{{model}}\", name: \"{{model}}\" },\n{{/models}}  ],\n  baseUrl: \"{{baseUrl}}\"\n}"},
		{Identifier: "opencode", Title: "OpenCode", Subtitle: "OpenCode configuration for PromptGate.", Icon: "mdi-code-json", Compatibility: "openai", ModelMode: "all", FilePaths: StringList{"~/.config/opencode/opencode.json"}, Template: "{\n  \"provider\": { \"promptgate\": { \"baseURL\": \"{{baseUrl}}\", \"apiKey\": \"{{token}}\", \"models\": {\n{{#models}}    \"{{model}}\": { \"name\": \"{{model}}\" },\n{{/models}}  } } }\n}"},
		{Identifier: "claude-code", Title: "Claude Code", Subtitle: "Claude Code environment configuration.", Icon: "mdi-alpha-c-circle-outline", Compatibility: "anthropic", ModelMode: "none", Template: "export ANTHROPIC_BASE_URL=\"{{anthropicBaseUrl}}\"\nexport ANTHROPIC_AUTH_TOKEN=\"{{token}}\""},
	}
	for i := range items {
		items[i].ID = uuid.New()
		if items[i].FilePaths == nil {
			items[i].FilePaths = StringList{}
		}
		items[i].Enabled = true
		items[i].Position = i
		items[i].CreatedAt = time.Now().UTC()
		items[i].UpdatedAt = items[i].CreatedAt
		items[i].Template = strings.ReplaceAll(items[i].Template, token, "{{token}}")
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Position < items[j].Position })
	return items
}
