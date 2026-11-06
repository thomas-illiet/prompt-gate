package groups

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"promptgate/backend/internal/domain/notifications"
)

func TestMembershipChangesAreAtomicAndNotifyOnlyRealChanges(t *testing.T) {
	service, db, user, provider, _ := newGroupTestService(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	ctx := context.Background()
	mail := notifications.NewService(db, true, "")
	if err := mail.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	service.SetNotificationService(mail)
	create := func(name string) GroupResponse {
		t.Helper()
		group, err := service.CreateGroup(ctx, CreateGroupInput{Name: name, DisplayName: strings.ToUpper(name), ProviderIDs: []string{provider.ID.String()}})
		if err != nil {
			t.Fatal(err)
		}
		return group
	}
	first, second := create("first"), create("second")
	for range 2 {
		if err := service.AddMember(ctx, first.ID.String(), user.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.ReplaceUserGroups(ctx, user.ID, []string{second.ID.String()}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReplaceUserGroups(ctx, user.ID, []string{second.ID.String(), second.ID.String()}); err != nil {
		t.Fatal(err)
	}
	var messages []notifications.Message
	if err := db.Order("created_at").Find(&messages).Error; err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || !strings.Contains(messages[1].MarkdownBody, "**Added:** SECOND") || !strings.Contains(messages[1].MarkdownBody, "**Removed:** FIRST") {
		t.Fatalf("membership notifications missing or duplicated: %#v", messages)
	}
	// A failed notification insert must roll back the membership transaction.
	if err := db.Migrator().DropTable(&notifications.Message{}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReplaceUserGroups(ctx, user.ID, []string{first.ID.String()}); err == nil {
		t.Fatal("fixture did not fail notification persistence")
	}
	memberships, err := service.ListUserGroups(ctx, user.ID)
	if err != nil || len(memberships) != 1 || memberships[0].ID != second.ID {
		t.Fatalf("partial membership change committed: %#v %v", memberships, err)
	}
	if err := service.RemoveMember(ctx, second.ID.String(), user.ID); err == nil {
		t.Fatal("failed notification did not abort removal")
	}
	service.SetNotificationService(nil)
	if err := service.RemoveMember(ctx, second.ID.String(), user.ID); err != nil {
		t.Fatal(err)
	}
	memberships, err = service.ListUserGroups(ctx, user.ID)
	if err != nil || len(memberships) != 0 {
		t.Fatal("membership removal not persisted", err)
	}
	for _, bad := range []string{"invalid", uuid.NewString()} {
		if err := service.AddMember(ctx, bad, user.ID); !errors.Is(err, ErrGroupNotFound) {
			t.Fatalf("missing group accepted: %v", err)
		}
		if err := service.RemoveMember(ctx, bad, user.ID); !errors.Is(err, ErrGroupNotFound) {
			t.Fatalf("missing group removal accepted: %v", err)
		}
		if _, err := service.ReplaceUserGroups(ctx, user.ID, []string{bad}); !errors.Is(err, ErrGroupNotFound) {
			t.Fatalf("missing replacement group accepted: %v", err)
		}
	}
}
