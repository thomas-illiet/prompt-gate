package faq

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
)

func TestFAQListPaginationSortingAndUpdate(t *testing.T) {
	service, ctx := newTestService(t), context.Background()
	for _, input := range []Input{
		{Question: " Charlie? ", Answer: "Charlie", Published: false},
		{Question: "Alpha?", Answer: "**Alpha**", Published: true},
		{Question: "Bravo?", Answer: "Bravo", Published: false},
	} {
		if _, err := service.Create(ctx, input); err != nil {
			t.Fatalf("create FAQ entry: %v", err)
		}
	}

	list, err := service.List(ctx, ListParams{SortBy: "question", SortDir: "ASC"})
	if err != nil {
		t.Fatalf("list FAQ entries: %v", err)
	}
	if list.Page != 1 || list.PageSize != 10 || list.Total != 3 || len(list.Items) != 3 {
		t.Fatalf("unexpected default pagination: %#v", list)
	}
	if got := []string{list.Items[0].Question, list.Items[1].Question, list.Items[2].Question}; strings.Join(got, ",") != "Alpha?,Bravo?,Charlie?" {
		t.Fatalf("unexpected question order: %#v", got)
	}
	if !strings.Contains(list.Items[0].RenderedHTML, "<strong>Alpha</strong>") {
		t.Fatalf("expected rendered answer, got %q", list.Items[0].RenderedHTML)
	}

	page, err := service.List(ctx, ListParams{Page: 2, PageSize: 1, SortBy: "question", SortDir: "desc"})
	if err != nil {
		t.Fatalf("list paginated FAQ entries: %v", err)
	}
	if page.Page != 2 || page.PageSize != 1 || len(page.Items) != 1 || page.Items[0].Question != "Bravo?" {
		t.Fatalf("unexpected second page: %#v", page)
	}

	capped, err := service.List(ctx, ListParams{Page: 1, PageSize: 101, SortBy: "position", SortDir: "asc"})
	if err != nil {
		t.Fatalf("list capped FAQ entries: %v", err)
	}
	if capped.PageSize != 100 {
		t.Fatalf("page size = %d, want 100", capped.PageSize)
	}

	for name, params := range map[string]ListParams{
		"unknown column": {SortBy: "missing", SortDir: "asc"},
		"unknown order":  {SortBy: "position", SortDir: "sideways"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.List(ctx, params); !errors.Is(err, ErrInvalidSort) {
				t.Fatalf("List() error = %v, want %v", err, ErrInvalidSort)
			}
		})
	}

	question, answer, published := " Updated question? ", "#Updated answer", true
	updated, err := service.Update(ctx, list.Items[2].ID.String(), UpdateInput{
		Question:  &question,
		Answer:    &answer,
		Published: &published,
	})
	if err != nil {
		t.Fatalf("update FAQ entry: %v", err)
	}
	if updated.Question != "Updated question?" || updated.Answer != "#Updated answer" || !updated.Published {
		t.Fatalf("unexpected updated entry: %#v", updated)
	}
	if !strings.Contains(updated.RenderedHTML, ">Updated answer</h1>") {
		t.Fatalf("unexpected updated HTML: %q", updated.RenderedHTML)
	}

	unchanged, err := service.Update(ctx, updated.ID.String(), UpdateInput{})
	if err != nil {
		t.Fatalf("update FAQ entry with empty patch: %v", err)
	}
	if unchanged.Question != updated.Question || unchanged.Answer != updated.Answer || unchanged.Published != updated.Published {
		t.Fatalf("empty update changed entry: %#v", unchanged)
	}
}

func TestFAQUpdateValidationAndMissingEntries(t *testing.T) {
	service, ctx := newTestService(t), context.Background()
	entry, err := service.Create(ctx, Input{Question: "Question?", Answer: "Answer"})
	if err != nil {
		t.Fatal(err)
	}

	tooLong := strings.Repeat("é", maxQuestionLength+1)
	if _, err := service.Create(ctx, Input{Question: tooLong, Answer: "answer"}); !errors.Is(err, ErrQuestionLength) {
		t.Fatalf("Create() error = %v, want %v", err, ErrQuestionLength)
	}
	blankQuestion := "  "
	if _, err := service.Update(ctx, entry.ID.String(), UpdateInput{Question: &blankQuestion}); !errors.Is(err, ErrQuestion) {
		t.Fatalf("Update() question error = %v, want %v", err, ErrQuestion)
	}
	blankAnswer := "\t"
	if _, err := service.Update(ctx, entry.ID.String(), UpdateInput{Answer: &blankAnswer}); !errors.Is(err, ErrAnswer) {
		t.Fatalf("Update() answer error = %v, want %v", err, ErrAnswer)
	}
	if _, err := service.Update(ctx, "invalid", UpdateInput{}); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("Update() invalid ID error = %v, want %v", err, ErrInvalidID)
	}
	missingID := uuid.NewString()
	if _, err := service.Get(ctx, missingID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() missing error = %v, want %v", err, ErrNotFound)
	}
	if _, err := service.Update(ctx, missingID, UpdateInput{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update() missing error = %v, want %v", err, ErrNotFound)
	}
}

func TestFAQMoveForwardBackwardAndRejectInvalidPositions(t *testing.T) {
	service, ctx := newTestService(t), context.Background()
	entries := make([]Response, 0, 3)
	for _, question := range []string{"First?", "Second?", "Third?"} {
		entry, err := service.Create(ctx, Input{Question: question, Answer: question})
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}

	if moved, err := service.Move(ctx, entries[0].ID.String(), 2); err != nil || moved.Position != 2 {
		t.Fatalf("move forward = %#v, %v", moved, err)
	}
	if moved, err := service.Move(ctx, entries[0].ID.String(), 0); err != nil || moved.Position != 0 {
		t.Fatalf("move backward = %#v, %v", moved, err)
	}
	if moved, err := service.Move(ctx, entries[1].ID.String(), 1); err != nil || moved.Position != 1 {
		t.Fatalf("move in place = %#v, %v", moved, err)
	}

	for _, position := range []int{-1, 3} {
		if _, err := service.Move(ctx, entries[0].ID.String(), position); !errors.Is(err, ErrInvalidPosition) {
			t.Fatalf("Move(%d) error = %v, want %v", position, err, ErrInvalidPosition)
		}
	}
	if _, err := service.Move(ctx, "invalid", 0); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("Move() invalid ID error = %v, want %v", err, ErrInvalidID)
	}
	missingID := uuid.NewString()
	if _, err := service.Move(ctx, missingID, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Move() missing error = %v, want %v", err, ErrNotFound)
	}
	if err := service.Delete(ctx, "invalid"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("Delete() invalid ID error = %v, want %v", err, ErrInvalidID)
	}
	if err := service.Delete(ctx, missingID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete() missing error = %v, want %v", err, ErrNotFound)
	}
}

type failingMarkdown struct {
	goldmark.Markdown
	err error
}

func (m failingMarkdown) Convert(_ []byte, _ io.Writer, _ ...parser.ParseOption) error {
	return m.err
}

func TestFAQPropagatesRenderingFailures(t *testing.T) {
	service, ctx := newTestService(t), context.Background()
	entry, err := service.Create(ctx, Input{Question: "Published?", Answer: "Answer", Published: true})
	if err != nil {
		t.Fatal(err)
	}
	renderErr := errors.New("render failed")
	service.markdown = failingMarkdown{Markdown: service.markdown, err: renderErr}

	if _, err := service.Render("answer"); !errors.Is(err, renderErr) {
		t.Fatalf("Render() error = %v, want wrapped render error", err)
	}
	if _, err := service.Get(ctx, entry.ID.String()); !errors.Is(err, renderErr) {
		t.Fatalf("Get() error = %v, want wrapped render error", err)
	}
	if _, err := service.List(ctx, ListParams{SortBy: "position", SortDir: "asc"}); !errors.Is(err, renderErr) {
		t.Fatalf("List() error = %v, want wrapped render error", err)
	}
	if _, err := service.ListPublished(ctx); !errors.Is(err, renderErr) {
		t.Fatalf("ListPublished() error = %v, want wrapped render error", err)
	}
	question := "Updated?"
	if _, err := service.Update(ctx, entry.ID.String(), UpdateInput{Question: &question}); !errors.Is(err, renderErr) {
		t.Fatalf("Update() error = %v, want wrapped render error", err)
	}
	if _, err := service.Move(ctx, entry.ID.String(), 0); !errors.Is(err, renderErr) {
		t.Fatalf("Move() error = %v, want wrapped render error", err)
	}
	if _, err := service.Create(ctx, Input{Question: "Another?", Answer: "Answer"}); !errors.Is(err, renderErr) {
		t.Fatalf("Create() error = %v, want wrapped render error", err)
	}
}

func TestFAQPropagatesDatabaseFailures(t *testing.T) {
	service, ctx := newTestService(t), context.Background()
	sqlDB, err := service.db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	validID := uuid.NewString()
	checks := map[string]func() error{
		"auto migrate": func() error { return service.AutoMigrate(ctx) },
		"list": func() error {
			_, err := service.List(ctx, ListParams{SortBy: "position", SortDir: "asc"})
			return err
		},
		"list published": func() error {
			_, err := service.ListPublished(ctx)
			return err
		},
		"get": func() error {
			_, err := service.Get(ctx, validID)
			return err
		},
		"create": func() error {
			_, err := service.Create(ctx, Input{Question: "Question?", Answer: "Answer"})
			return err
		},
		"update": func() error {
			_, err := service.Update(ctx, validID, UpdateInput{})
			return err
		},
		"move": func() error {
			_, err := service.Move(ctx, validID, 0)
			return err
		},
		"delete": func() error { return service.Delete(ctx, validID) },
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			if err := check(); err == nil {
				t.Fatal("expected database error")
			}
		})
	}
}
