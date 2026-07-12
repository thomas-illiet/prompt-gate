package proxy

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestListPromptsPaginatesSearchesAndIsolatesUsers verifies list prompts paginates searches and isolates users.
func TestListPromptsPaginatesSearchesAndIsolatesUsers(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"

	seedProxyInteraction(t, db, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Alpha first", "gpt-5", now.Add(-3*time.Hour), 1, 2)
	seedProxyInteraction(t, db, userID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "Beta prompt", "gpt-5", now.Add(-2*time.Hour), 3, 4)
	seedProxyInteraction(t, db, userID, "cccccccc-cccc-cccc-cccc-cccccccccccc", "Alpha newest", "gpt-5", now.Add(-1*time.Hour), 5, 6)
	seedProxyInteraction(t, db, "22222222-2222-2222-2222-222222222222", "dddddddd-dddd-dddd-dddd-dddddddddddd", "Alpha hidden", "gpt-5", now, 100, 200)

	result, err := service.ListPrompts(context.Background(), userID, PromptListParams{
		Page:     1,
		PageSize: 1,
		Search:   "alpha",
	})
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}

	if result.Total != 2 || len(result.Items) != 1 {
		t.Fatalf("unexpected page result: %#v", result)
	}
	if result.Items[0].Prompt != "Alpha newest" {
		t.Fatalf("expected newest alpha prompt, got %#v", result.Items[0])
	}
	if result.Items[0].InputTokens != 5 || result.Items[0].OutputTokens != 6 || result.Items[0].TotalTokens != 11 {
		t.Fatalf("expected attached token totals, got %#v", result.Items[0])
	}
	if result.Items[0].DurationMs == nil || *result.Items[0].DurationMs != 90000 {
		t.Fatalf("expected attached duration, got %#v", result.Items[0].DurationMs)
	}
}

// TestListPromptsSortsByDuration verifies list prompts sorts by duration.
func TestListPromptsSortsByDuration(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userID := "11111111-1111-1111-1111-111111111111"
	longStartedAt := now.Add(-3 * time.Hour)
	shortStartedAt := now.Add(-2 * time.Hour)

	seedProxyInteraction(t, db, userID, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Long prompt", "gpt-5", longStartedAt, 1, 2)
	seedProxyInteraction(t, db, userID, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "Pending prompt", "gpt-5", now.Add(-1*time.Hour), 3, 4)
	seedProxyInteraction(t, db, userID, "cccccccc-cccc-cccc-cccc-cccccccccccc", "Short prompt", "gpt-5", shortStartedAt, 5, 6)

	longEndedAt := longStartedAt.Add(3 * time.Minute)
	shortEndedAt := shortStartedAt.Add(30 * time.Second)
	setInterceptionEndedAt(t, db, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", &longEndedAt)
	setInterceptionEndedAt(t, db, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", nil)
	setInterceptionEndedAt(t, db, "cccccccc-cccc-cccc-cccc-cccccccccccc", &shortEndedAt)

	result, err := service.ListPrompts(context.Background(), userID, PromptListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "durationMs",
		SortDir:  "asc",
	})
	if err != nil {
		t.Fatalf("sort prompts by duration: %v", err)
	}

	if got := []string{result.Items[0].Prompt, result.Items[1].Prompt, result.Items[2].Prompt}; fmt.Sprint(got) != "[Short prompt Long prompt Pending prompt]" {
		t.Fatalf("unexpected duration order: %v", got)
	}
	if result.Items[2].DurationMs != nil {
		t.Fatalf("expected pending prompt duration to be nil, got %v", *result.Items[2].DurationMs)
	}
}

// TestListAdminPromptsSearchesFiltersIdentifiesUsersAndSortsTokens verifies list admin prompts searches filters identifies users and sorts tokens.
func TestListAdminPromptsSearchesFiltersIdentifiesUsersAndSortsTokens(t *testing.T) {
	db, service := newProxyServiceTestDB(t)
	now := time.Date(2026, 1, 30, 15, 0, 0, 0, time.UTC)
	userOne := "11111111-1111-1111-1111-111111111111"
	userTwo := "22222222-2222-2222-2222-222222222222"

	seedProxyInteraction(t, db, userOne, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Alpha first", "gpt-5", now.Add(-3*time.Hour), 10, 2)
	seedProxyInteraction(t, db, userTwo, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "Beta prompt", "gpt-4o", now.Add(-2*time.Hour), 5, 1)
	seedProxyInteraction(t, db, userTwo, "cccccccc-cccc-cccc-cccc-cccccccccccc", "Alpha newest", "gpt-5", now.Add(-1*time.Hour), 50, 1)
	setInterceptionClientIP(t, db, "cccccccc-cccc-cccc-cccc-cccccccccccc", "198.51.100.7")

	result, err := service.ListAdminPrompts(context.Background(), AdminPromptListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "createdAt",
		SortDir:  "desc",
	})
	if err != nil {
		t.Fatalf("list admin prompts: %v", err)
	}

	if result.Total != 3 || len(result.Items) != 3 {
		t.Fatalf("unexpected admin prompt result: %#v", result)
	}
	if result.Items[0].Prompt != "Alpha newest" || result.Items[0].UserID != userTwo {
		t.Fatalf("expected newest prompt for user two, got %#v", result.Items[0])
	}
	if result.Items[0].UserName != "Two" || result.Items[0].UserEmail != "two@example.com" || result.Items[0].UserPreferredUsername != "two" {
		t.Fatalf("expected user identity on prompt row, got %#v", result.Items[0])
	}
	if result.Items[0].ClientIP != "198.51.100.7" {
		t.Fatalf("expected admin prompt client IP, got %#v", result.Items[0])
	}
	if result.Items[0].InputTokens != 50 || result.Items[0].OutputTokens != 1 || result.Items[0].TotalTokens != 51 {
		t.Fatalf("expected attached token totals, got %#v", result.Items[0])
	}
	if result.Items[0].DurationMs == nil || *result.Items[0].DurationMs != 90000 {
		t.Fatalf("expected attached duration, got %#v", result.Items[0].DurationMs)
	}

	filtered, err := service.ListAdminPrompts(context.Background(), AdminPromptListParams{
		Page:     1,
		PageSize: 10,
		Search:   "alpha",
		UserID:   userOne,
	})
	if err != nil {
		t.Fatalf("filter admin prompts: %v", err)
	}
	if filtered.Total != 1 || filtered.Items[0].Prompt != "Alpha first" {
		t.Fatalf("expected filtered prompt for user one, got %#v", filtered)
	}

	sorted, err := service.ListAdminPrompts(context.Background(), AdminPromptListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "totalTokens",
		SortDir:  "asc",
	})
	if err != nil {
		t.Fatalf("sort admin prompts by tokens: %v", err)
	}
	if sorted.Items[0].Prompt != "Beta prompt" || sorted.Items[0].TotalTokens != 6 {
		t.Fatalf("expected lowest token prompt first, got %#v", sorted.Items[0])
	}

	longEndedAt := now.Add(-3 * time.Hour).Add(4 * time.Minute)
	shortEndedAt := now.Add(-2 * time.Hour).Add(20 * time.Second)
	setInterceptionEndedAt(t, db, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", &longEndedAt)
	setInterceptionEndedAt(t, db, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", &shortEndedAt)
	setInterceptionEndedAt(t, db, "cccccccc-cccc-cccc-cccc-cccccccccccc", nil)

	durationSorted, err := service.ListAdminPrompts(context.Background(), AdminPromptListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "durationMs",
		SortDir:  "desc",
	})
	if err != nil {
		t.Fatalf("sort admin prompts by duration: %v", err)
	}
	if got := []string{durationSorted.Items[0].Prompt, durationSorted.Items[1].Prompt, durationSorted.Items[2].Prompt}; fmt.Sprint(got) != "[Alpha first Beta prompt Alpha newest]" {
		t.Fatalf("unexpected admin duration order: %v", got)
	}
	if durationSorted.Items[2].DurationMs != nil {
		t.Fatalf("expected pending admin prompt duration to be nil, got %v", *durationSorted.Items[2].DurationMs)
	}
}

// TestListAdminPromptsRejectsInvalidSort verifies list admin prompts rejects invalid sort.
func TestListAdminPromptsRejectsInvalidSort(t *testing.T) {
	_, service := newProxyServiceTestDB(t)
	_, err := service.ListAdminPrompts(context.Background(), AdminPromptListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "unknown",
		SortDir:  "desc",
	})
	if !errors.Is(err, ErrInvalidSort) {
		t.Fatalf("expected invalid sort error, got %v", err)
	}
}
