package setupguide

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type legacySetupGuide struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Identifier    string     `gorm:"uniqueIndex;not null"`
	Title         string     `gorm:"not null"`
	Subtitle      string     `gorm:"not null"`
	Icon          string     `gorm:"not null"`
	Compatibility string     `gorm:"not null;index"`
	ModelMode     string     `gorm:"not null"`
	FilePaths     StringList `gorm:"type:jsonb;not null"`
	Template      string     `gorm:"type:text;not null"`
	Enabled       bool       `gorm:"not null;index"`
	Position      int        `gorm:"not null;index"`
}

func (legacySetupGuide) TableName() string { return "setup_guides" }

func testService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return service
}

func TestAutoMigrateSeedsDefaultsOnlyWhenEmpty(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	items, err := s.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 12 {
		t.Fatalf("expected 12 defaults, got %d", len(items))
	}
	for _, item := range items {
		if got, want := item.SyntaxLanguage, defaultSyntaxLanguages[item.Identifier]; got != want {
			t.Fatalf("default %q syntax language = %q, want %q", item.Identifier, got, want)
		}
	}
	if err := s.Delete(ctx, items[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	items, _ = s.List(ctx, false)
	if len(items) != 11 {
		t.Fatalf("deleted default was recreated: %d", len(items))
	}
}

func TestCRUDAndReorder(t *testing.T) {
	s := testService(t)
	ctx := context.Background()
	existing, _ := s.List(ctx, false)
	created, err := s.Create(ctx, Input{Identifier: "custom", Title: "Custom", Subtitle: "Example", Icon: "mdi-code-tags", Compatibility: "both", ModelMode: "single", Template: "{{providerDisplayName}}: {{model}}", Enabled: true, Position: 12})
	if err != nil {
		t.Fatal(err)
	}
	if created.SyntaxLanguage != SyntaxLanguagePlaintext {
		t.Fatalf("default syntax language = %q", created.SyntaxLanguage)
	}
	created.Title = "Updated"
	updated, err := s.Update(ctx, created.ID, Input{Identifier: created.Identifier, Title: created.Title, Subtitle: created.Subtitle, Icon: created.Icon, Compatibility: created.Compatibility, ModelMode: created.ModelMode, SyntaxLanguage: SyntaxLanguagePython, FilePaths: created.FilePaths, Template: created.Template, Enabled: false, Position: created.Position})
	if err != nil || updated.Title != "Updated" || updated.Enabled || updated.SyntaxLanguage != SyntaxLanguagePython {
		t.Fatalf("unexpected update: %#v %v", updated, err)
	}
	ids := []uuid.UUID{created.ID}
	for _, item := range existing {
		ids = append(ids, item.ID)
	}
	if err := s.Reorder(ctx, ids); err != nil {
		t.Fatal(err)
	}
	ordered, _ := s.List(ctx, false)
	if ordered[0].ID != created.ID {
		t.Fatal("reorder did not persist")
	}
	if err := s.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
}

func TestAutoMigrateBackfillsSyntaxLanguageOnlyOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&legacySetupGuide{}); err != nil {
		t.Fatal(err)
	}
	legacy := []legacySetupGuide{
		{ID: uuid.New(), Identifier: "curl", Title: "curl", Subtitle: "Legacy curl", Icon: "mdi-console-line", Compatibility: CompatibilityOpenAI, ModelMode: ModelModeSingle, FilePaths: StringList{}, Template: "curl {{baseUrl}}", Enabled: true},
		{ID: uuid.New(), Identifier: "custom", Title: "Custom", Subtitle: "Legacy custom", Icon: "mdi-code-tags", Compatibility: CompatibilityBoth, ModelMode: ModelModeSingle, FilePaths: StringList{}, Template: "{{baseUrl}}", Enabled: true, Position: 1},
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}

	service := NewService(db)
	ctx := context.Background()
	if err := service.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := service.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].SyntaxLanguage != SyntaxLanguageBash || items[1].SyntaxLanguage != SyntaxLanguagePlaintext {
		t.Fatalf("unexpected migrated guides: %#v", items)
	}

	if err := db.Model(&SetupGuide{}).Where("identifier = ?", "curl").Update("syntax_language", SyntaxLanguageLua).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	item, err := service.Get(ctx, legacy[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.SyntaxLanguage != SyntaxLanguageLua {
		t.Fatalf("second migration overwrote syntax language: %q", item.SyntaxLanguage)
	}
}

func TestCreateRejectsInvalidSyntaxLanguage(t *testing.T) {
	s := testService(t)
	_, err := s.Create(context.Background(), Input{Identifier: "invalid-syntax", Title: "Invalid", Icon: "mdi-code-tags", Compatibility: CompatibilityBoth, ModelMode: ModelModeSingle, SyntaxLanguage: "brainfuck", Template: "{{baseUrl}}", Position: 12})
	if err == nil || err.Error() != "invalid syntax language" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateTemplate(t *testing.T) {
	valid := "{{baseUrl}} {{#models}}{{model}} {{/models}}"
	if err := ValidateTemplate(valid); err != nil {
		t.Fatalf("valid template rejected: %v", err)
	}
	for _, value := range []string{"{{secret}}", "{{#model}}{{model}}{{/model}}", "{{#models}}{{model}}"} {
		if ValidateTemplate(value) == nil {
			t.Fatalf("invalid template accepted: %s", value)
		}
	}
}
