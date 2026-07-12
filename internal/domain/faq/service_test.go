package faq

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return service
}

func TestFAQCRUDPublicationAndOrdering(t *testing.T) {
	service, ctx := newTestService(t), context.Background()
	first, err := service.Create(ctx, Input{Question: " First? ", Answer: "**First**", Published: true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx, Input{Question: "Second?", Answer: "Second", Published: false})
	if err != nil {
		t.Fatal(err)
	}
	third, err := service.Create(ctx, Input{Question: "Third?", Answer: "Third", Published: true})
	if err != nil {
		t.Fatal(err)
	}
	if first.Position != 0 || second.Position != 1 || third.Position != 2 {
		t.Fatalf("unexpected positions: %d %d %d", first.Position, second.Position, third.Position)
	}
	if _, err := service.Move(ctx, third.ID.String(), 0); err != nil {
		t.Fatal(err)
	}
	published, err := service.ListPublished(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(published) != 2 || published[0].Question != "Third?" || published[1].Question != "First?" {
		t.Fatalf("unexpected published FAQ: %#v", published)
	}
	if err := service.Delete(ctx, third.ID.String()); err != nil {
		t.Fatal(err)
	}
	got, err := service.Get(ctx, first.ID.String())
	if err != nil || got.Position != 0 {
		t.Fatalf("expected compacted first entry, got %#v, %v", got, err)
	}
}

func TestFAQValidationAndMissingEntry(t *testing.T) {
	service, ctx := newTestService(t), context.Background()
	if _, err := service.Create(ctx, Input{Question: "", Answer: "answer"}); !errors.Is(err, ErrQuestion) {
		t.Fatalf("expected question error, got %v", err)
	}
	if _, err := service.Create(ctx, Input{Question: "question", Answer: ""}); !errors.Is(err, ErrAnswer) {
		t.Fatalf("expected answer error, got %v", err)
	}
	if _, err := service.Get(ctx, "not-a-uuid"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected invalid id, got %v", err)
	}
}

func TestRenderMarkdownSanitizesUnsafeContent(t *testing.T) {
	html, err := newTestService(t).Render("# Title\n\n**bold** [bad](javascript:alert(1))\n\n<script>alert(1)</script>\n\n- item\n\n```go\nfmt.Println()\n```")
	if err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"<script", "javascript:"} {
		if strings.Contains(strings.ToLower(html), unsafe) {
			t.Fatalf("unsafe output %q: %s", unsafe, html)
		}
	}
	for _, expected := range []string{"<h1", "<strong>bold</strong>", "<li>item</li>", "<code"} {
		if !strings.Contains(html, expected) {
			t.Fatalf("missing %q in %s", expected, html)
		}
	}
}

func TestRenderMarkdownAcceptsCompactHeadings(t *testing.T) {
	html, err := newTestService(t).Render("#patate\n\n##Sous-titre")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"<h1", ">patate</h1>", "<h2", ">Sous-titre</h2>"} {
		if !strings.Contains(html, expected) {
			t.Fatalf("missing %q in %s", expected, html)
		}
	}
}
