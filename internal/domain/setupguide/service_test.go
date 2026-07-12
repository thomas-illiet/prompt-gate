package setupguide

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
	created.Title = "Updated"
	updated, err := s.Update(ctx, created.ID, Input{Identifier: created.Identifier, Title: created.Title, Subtitle: created.Subtitle, Icon: created.Icon, Compatibility: created.Compatibility, ModelMode: created.ModelMode, FilePaths: created.FilePaths, Template: created.Template, Enabled: false, Position: created.Position})
	if err != nil || updated.Title != "Updated" || updated.Enabled {
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
