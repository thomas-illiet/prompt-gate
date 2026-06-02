package groups

import (
	"context"
	"errors"
	"testing"
	"time"

	"promptgate/backend/internal/domain/auth"
	"promptgate/backend/internal/domain/provider"
	"promptgate/backend/internal/domain/users"
	"promptgate/backend/internal/platform/configevents"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type recordingNotifier struct {
	domains []string
}

// Notify records the notification domain for assertions.
func (r *recordingNotifier) Notify(_ context.Context, domain string) {
	r.domains = append(r.domains, domain)
}

// newGroupTestService creates group test service.
func newGroupTestService(t *testing.T) (*Service, *gorm.DB, users.User, provider.Provider, provider.Provider) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&users.User{}, &provider.Provider{}); err != nil {
		t.Fatalf("migrate dependencies: %v", err)
	}
	service := NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate groups: %v", err)
	}

	user := users.User{
		ID:                uuid.NewString(),
		ExternalSub:       "oidc-sub",
		Email:             "user@example.com",
		PreferredUsername: "user",
		Name:              "User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       time.Now().UTC(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	openai := provider.Provider{
		ID:          uuid.New(),
		Name:        "openai-main",
		DisplayName: "OpenAI Main",
		Type:        provider.ProviderTypeOpenAI,
		BaseURL:     "https://api.openai.com/v1",
		Enabled:     true,
	}
	anthropic := provider.Provider{
		ID:          uuid.New(),
		Name:        "anthropic-main",
		DisplayName: "Anthropic Main",
		Type:        provider.ProviderTypeAnthropic,
		BaseURL:     "https://api.anthropic.com",
		Enabled:     true,
	}
	if err := db.Create(&openai).Error; err != nil {
		t.Fatalf("create openai provider: %v", err)
	}
	if err := db.Create(&anthropic).Error; err != nil {
		t.Fatalf("create anthropic provider: %v", err)
	}
	return service, db, user, openai, anthropic
}

// TestSnapshotRequiresModelRegexAndHonorsProviderScope verifies snapshot requires model regex and honors provider scope.
func TestSnapshotRequiresModelRegexAndHonorsProviderScope(t *testing.T) {
	service, _, user, openai, anthropic := newGroupTestService(t)
	ctx := context.Background()

	group, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:          "engineering",
		DisplayName:   "Engineering",
		ProviderIDs:   []string{openai.ID.String()},
		ModelPatterns: []string{`^gpt-5`},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := service.ReplaceUserGroups(ctx, user.ID, []string{group.ID.String()}); err != nil {
		t.Fatalf("replace user groups: %v", err)
	}

	snapshot, err := service.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	store := NewSnapshotStore(service)
	if err := store.SetSnapshot(snapshot); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}

	if !store.KnownProvider(openai.Name) || !store.KnownProvider(anthropic.Name) {
		t.Fatalf("expected known providers in snapshot: %#v", store.Snapshot().KnownProviders)
	}
	if store.Allows(user.ID, openai.Name, "") {
		t.Fatal("expected model-less provider request to be denied")
	}
	if !store.Allows(user.ID, openai.Name, "gpt-5-mini") {
		t.Fatal("expected provider and model regex grant to allow matching model")
	}
	if store.Allows(user.ID, anthropic.Name, "gpt-5-mini") {
		t.Fatal("expected provider-scoped model regex to deny another provider")
	}
	if store.Allows(user.ID, anthropic.Name, "claude-sonnet-4") {
		t.Fatal("expected unmatched provider/model request to be denied")
	}
	if store.Allows(uuid.NewString(), openai.Name, "") {
		t.Fatal("expected user without group to be denied")
	}
}

// TestSetSnapshotRejectsLegacyAggregatedAccess verifies set snapshot rejects legacy aggregated access.
func TestSetSnapshotRejectsLegacyAggregatedAccess(t *testing.T) {
	store := NewSnapshotStore(nil)

	err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
		ProviderTypes: map[string]provider.ProviderType{
			"openai": provider.ProviderTypeOpenAI,
		},
		Users: map[string]UserAccess{
			"user-id": {
				Providers:     []string{"openai"},
				ModelPatterns: []string{`^gpt-5`},
			},
		},
	})

	if !errors.Is(err, ErrLegacySnapshot) {
		t.Fatalf("expected ErrLegacySnapshot, got %v", err)
	}
	if store.Allows("user-id", "openai", "gpt-5-mini") {
		t.Fatal("legacy snapshot should not update access rules")
	}
}

// TestSetSnapshotRejectsMissingProviderTypes verifies set snapshot rejects missing provider types.
func TestSetSnapshotRejectsMissingProviderTypes(t *testing.T) {
	store := NewSnapshotStore(nil)

	err := store.SetSnapshot(Snapshot{
		KnownProviders: []string{"openai"},
		Users: map[string]UserAccess{
			"user-id": {
				Rules: []AccessRule{{
					Providers:     []string{"openai"},
					ModelPatterns: []string{`^gpt-5`},
				}},
			},
		},
	})

	if !errors.Is(err, ErrLegacySnapshot) {
		t.Fatalf("expected ErrLegacySnapshot, got %v", err)
	}
	if store.Allows("user-id", "openai", "gpt-5-mini") {
		t.Fatal("legacy snapshot should not update access rules")
	}
}

// TestSnapshotDefaultsMissingModelRegexToAllModels verifies snapshot defaults missing model regex to all models.
func TestSnapshotDefaultsMissingModelRegexToAllModels(t *testing.T) {
	service, _, user, openai, anthropic := newGroupTestService(t)
	ctx := context.Background()

	group, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:        "platform",
		DisplayName: "Platform",
		ProviderIDs: []string{openai.ID.String(), anthropic.ID.String()},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := service.ReplaceUserGroups(ctx, user.ID, []string{group.ID.String()}); err != nil {
		t.Fatalf("replace user groups: %v", err)
	}

	store := NewSnapshotStore(service)
	if err := store.Refresh(ctx); err != nil {
		t.Fatalf("refresh snapshot: %v", err)
	}

	for _, tc := range []struct {
		providerName string
		model        string
	}{
		{providerName: openai.Name, model: "gpt-5-mini"},
		{providerName: openai.Name, model: "custom-openai-model"},
		{providerName: anthropic.Name, model: "claude-sonnet-4"},
		{providerName: anthropic.Name, model: "another-model"},
	} {
		if !store.Allows(user.ID, tc.providerName, tc.model) {
			t.Fatalf("expected default all-model pattern to allow %s/%s", tc.providerName, tc.model)
		}
	}
}

// TestSnapshotCombinesMultipleGroupsWithUnionSemantics verifies snapshot combines multiple groups with union semantics.
func TestSnapshotCombinesMultipleGroupsWithUnionSemantics(t *testing.T) {
	service, db, user, openai, anthropic := newGroupTestService(t)
	ctx := context.Background()
	serviceUser := users.User{
		ID:                uuid.NewString(),
		ExternalSub:       "service-sub",
		PreferredUsername: "automation",
		Name:              "Automation",
		Type:              auth.UserTypeService,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       time.Now().UTC(),
	}
	if err := db.Create(&serviceUser).Error; err != nil {
		t.Fatalf("create service user: %v", err)
	}

	openAIGroup, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:          "openai-access",
		DisplayName:   "OpenAI Access",
		ProviderIDs:   []string{openai.ID.String()},
		ModelPatterns: []string{`^gpt-5`},
	})
	if err != nil {
		t.Fatalf("create openai group: %v", err)
	}
	modelGroup, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:          "gpt-five",
		DisplayName:   "GPT Five",
		ProviderIDs:   []string{anthropic.ID.String()},
		ModelPatterns: []string{`^gpt-5`},
	})
	if err != nil {
		t.Fatalf("create model group: %v", err)
	}
	if _, err := service.ReplaceUserGroups(ctx, user.ID, []string{openAIGroup.ID.String(), modelGroup.ID.String()}); err != nil {
		t.Fatalf("replace user groups: %v", err)
	}
	if _, err := service.ReplaceUserGroups(ctx, serviceUser.ID, []string{modelGroup.ID.String()}); err != nil {
		t.Fatalf("replace service account groups: %v", err)
	}

	store := NewSnapshotStore(service)
	if err := store.Refresh(ctx); err != nil {
		t.Fatalf("refresh snapshot: %v", err)
	}

	if store.Allows(user.ID, openai.Name, "any-model") {
		t.Fatal("expected OpenAI non-matching model to be denied")
	}
	if !store.Allows(user.ID, openai.Name, "gpt-5-mini") {
		t.Fatal("expected model regex group to allow OpenAI gpt-5 model")
	}
	if !store.Allows(user.ID, anthropic.Name, "gpt-5-mini") {
		t.Fatal("expected second group regex grant to allow Anthropic gpt-5 model")
	}
	if store.Allows(user.ID, anthropic.Name, "claude-sonnet-4") {
		t.Fatal("expected Anthropic non-matching model to be denied")
	}
	if !store.Allows(serviceUser.ID, anthropic.Name, "gpt-5-mini") {
		t.Fatal("expected service account membership to use the same model grants")
	}
}

// TestUserAccessHonorsProviderScopedModelRegex verifies user access honors provider scoped model regex.
func TestUserAccessHonorsProviderScopedModelRegex(t *testing.T) {
	service, _, user, openai, anthropic := newGroupTestService(t)
	ctx := context.Background()

	group, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:          "engineering",
		DisplayName:   "Engineering",
		ProviderIDs:   []string{openai.ID.String()},
		ModelPatterns: []string{`^gpt-5`},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := service.ReplaceUserGroups(ctx, user.ID, []string{group.ID.String()}); err != nil {
		t.Fatalf("replace user groups: %v", err)
	}

	access, err := service.UserAccess(ctx, user.ID)
	if err != nil {
		t.Fatalf("user access: %v", err)
	}

	if len(access.Providers) != 1 || access.Providers[0] != openai.Name {
		t.Fatalf("expected OpenAI provider access, got %#v", access.Providers)
	}
	if !access.Allows(openai.Name, "gpt-5-mini") {
		t.Fatal("expected matching OpenAI model to be allowed")
	}
	if access.Allows(openai.Name, "gpt-4.1") {
		t.Fatal("expected non-matching OpenAI model to be denied")
	}
	if access.Allows(anthropic.Name, "gpt-5-mini") {
		t.Fatal("expected matching model on another provider to be denied")
	}
}

// TestUserAccessCombinesGroupsWithoutCrossProduct verifies user access combines groups without cross product.
func TestUserAccessCombinesGroupsWithoutCrossProduct(t *testing.T) {
	service, _, user, openai, anthropic := newGroupTestService(t)
	ctx := context.Background()

	openAIGroup, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:          "openai-access",
		DisplayName:   "OpenAI Access",
		ProviderIDs:   []string{openai.ID.String()},
		ModelPatterns: []string{`^gpt-5`},
	})
	if err != nil {
		t.Fatalf("create openai group: %v", err)
	}
	anthropicGroup, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:          "anthropic-access",
		DisplayName:   "Anthropic Access",
		ProviderIDs:   []string{anthropic.ID.String()},
		ModelPatterns: []string{`^claude`},
	})
	if err != nil {
		t.Fatalf("create anthropic group: %v", err)
	}
	if _, err := service.ReplaceUserGroups(ctx, user.ID, []string{openAIGroup.ID.String(), anthropicGroup.ID.String()}); err != nil {
		t.Fatalf("replace user groups: %v", err)
	}

	access, err := service.UserAccess(ctx, user.ID)
	if err != nil {
		t.Fatalf("user access: %v", err)
	}

	if !access.Allows(openai.Name, "gpt-5-mini") {
		t.Fatal("expected OpenAI GPT-5 model to be allowed")
	}
	if !access.Allows(anthropic.Name, "claude-sonnet-4") {
		t.Fatal("expected Anthropic Claude model to be allowed")
	}
	if access.Allows(openai.Name, "claude-sonnet-4") {
		t.Fatal("expected Anthropic regex not to cross-apply to OpenAI")
	}
	if access.Allows(anthropic.Name, "gpt-5-mini") {
		t.Fatal("expected OpenAI regex not to cross-apply to Anthropic")
	}
}

// TestUserAccessHonorsProviderScopedExcludedModelRegex verifies excluded patterns deny matching models for the scoped provider.
func TestUserAccessHonorsProviderScopedExcludedModelRegex(t *testing.T) {
	service, _, user, openai, anthropic := newGroupTestService(t)
	ctx := context.Background()

	group, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:                  "engineering",
		DisplayName:           "Engineering",
		ProviderIDs:           []string{openai.ID.String()},
		ModelPatterns:         []string{`.*`},
		ExcludedModelPatterns: []string{`^bge`},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := service.ReplaceUserGroups(ctx, user.ID, []string{group.ID.String()}); err != nil {
		t.Fatalf("replace user groups: %v", err)
	}

	access, err := service.UserAccess(ctx, user.ID)
	if err != nil {
		t.Fatalf("user access: %v", err)
	}

	if !access.Allows(openai.Name, "gpt-5-mini") {
		t.Fatal("expected non-excluded OpenAI model to be allowed")
	}
	if access.Allows(openai.Name, "bge-m3") {
		t.Fatal("expected excluded OpenAI model to be denied")
	}
	if access.Allows(anthropic.Name, "bge-m3") {
		t.Fatal("expected excluded pattern not to cross-apply to another provider")
	}
}

// TestCreateGroupStoresModelPatternsWithPatternTypes verifies allow and exclude patterns share one table.
func TestCreateGroupStoresModelPatternsWithPatternTypes(t *testing.T) {
	service, db, _, openai, _ := newGroupTestService(t)
	ctx := context.Background()

	group, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:                  "engineering",
		DisplayName:           "Engineering",
		ProviderIDs:           []string{openai.ID.String()},
		ModelPatterns:         []string{`.*`},
		ExcludedModelPatterns: []string{`^bge`},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	var records []GroupModelPattern
	if err := db.Where("group_id = ?", group.ID).Order("pattern_type ASC").Find(&records).Error; err != nil {
		t.Fatalf("load model patterns: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 model pattern rows, got %#v", records)
	}
	values := map[GroupModelPatternType]string{}
	for _, record := range records {
		values[record.PatternType] = record.Pattern
	}
	if values[GroupModelPatternTypeAllow] != `.*` {
		t.Fatalf("expected allow pattern in access_group_model_patterns, got %#v", values)
	}
	if values[GroupModelPatternTypeExclude] != `^bge` {
		t.Fatalf("expected exclude pattern in access_group_model_patterns, got %#v", values)
	}
}

// TestUserAccessReturnsEmptyForUserWithoutGroups verifies user access returns empty for user without groups.
func TestUserAccessReturnsEmptyForUserWithoutGroups(t *testing.T) {
	service, _, user, openai, _ := newGroupTestService(t)

	access, err := service.UserAccess(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("user access: %v", err)
	}

	if len(access.Providers) != 0 || len(access.ModelPatterns) != 0 || len(access.ExcludedModelPatterns) != 0 || len(access.Rules) != 0 {
		t.Fatalf("expected empty access, got %#v", access)
	}
	if access.Allows(openai.Name, "gpt-5-mini") {
		t.Fatal("expected user without groups to be denied")
	}
}

// TestCreateGroupRejectsInvalidRegex verifies create group rejects invalid regex.
func TestCreateGroupRejectsInvalidRegex(t *testing.T) {
	service, _, _, openai, _ := newGroupTestService(t)
	_, err := service.CreateGroup(context.Background(), CreateGroupInput{
		Name:          "engineering",
		DisplayName:   "Engineering",
		ProviderIDs:   []string{openai.ID.String()},
		ModelPatterns: []string{"["},
	})
	if !errors.Is(err, ErrInvalidRegex) {
		t.Fatalf("expected ErrInvalidRegex, got %v", err)
	}

	_, err = service.CreateGroup(context.Background(), CreateGroupInput{
		Name:                  "blocked",
		DisplayName:           "Blocked",
		ProviderIDs:           []string{openai.ID.String()},
		ExcludedModelPatterns: []string{"["},
	})
	if !errors.Is(err, ErrInvalidRegex) {
		t.Fatalf("expected ErrInvalidRegex for excluded pattern, got %v", err)
	}
}

// TestListGroupsPagedSortsByComputedCounts verifies list groups paged sorts by computed counts.
func TestListGroupsPagedSortsByComputedCounts(t *testing.T) {
	service, _, _, openai, anthropic := newGroupTestService(t)
	ctx := context.Background()

	oneProvider, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:        "one-provider",
		DisplayName: "One Provider",
		ProviderIDs: []string{openai.ID.String()},
	})
	if err != nil {
		t.Fatalf("create one-provider group: %v", err)
	}
	twoProviders, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:        "two-providers",
		DisplayName: "Two Providers",
		ProviderIDs: []string{openai.ID.String(), anthropic.ID.String()},
	})
	if err != nil {
		t.Fatalf("create two-providers group: %v", err)
	}

	result, err := service.ListGroupsPaged(ctx, ListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "providerCount",
		SortDir:  "desc",
	})
	if err != nil {
		t.Fatalf("list groups sorted by provider count: %v", err)
	}
	if len(result.Items) < 2 {
		t.Fatalf("expected two groups, got %#v", result.Items)
	}
	if result.Items[0].ID != twoProviders.ID || result.Items[1].ID != oneProvider.ID {
		t.Fatalf("unexpected provider count order: %#v", result.Items)
	}

	result, err = service.ListGroupsPaged(ctx, ListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "modelPatternCount",
		SortDir:  "asc",
	})
	if err != nil {
		t.Fatalf("list groups sorted by model pattern count: %v", err)
	}

	result, err = service.ListGroupsPaged(ctx, ListParams{
		Page:     1,
		PageSize: 10,
		SortBy:   "memberCount",
		SortDir:  "asc",
	})
	if err != nil {
		t.Fatalf("list groups sorted by member count: %v", err)
	}
}

// TestCreateGroupRequiresDisplayNameAndProvider verifies create group requires display name and provider.
func TestCreateGroupRequiresDisplayNameAndProvider(t *testing.T) {
	service, _, _, openai, _ := newGroupTestService(t)
	ctx := context.Background()

	_, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:        "engineering",
		ProviderIDs: []string{openai.ID.String()},
	})
	if !errors.Is(err, ErrInvalidDisplayName) {
		t.Fatalf("expected ErrInvalidDisplayName, got %v", err)
	}

	_, err = service.CreateGroup(ctx, CreateGroupInput{
		Name:        "engineering",
		DisplayName: "Engineering",
	})
	if !errors.Is(err, ErrProviderRequired) {
		t.Fatalf("expected ErrProviderRequired, got %v", err)
	}
}

// TestCreateGroupDefaultsEmptyModelPatternsToAllModels verifies create group defaults empty model patterns to all models.
func TestCreateGroupDefaultsEmptyModelPatternsToAllModels(t *testing.T) {
	service, _, _, openai, _ := newGroupTestService(t)
	group, err := service.CreateGroup(context.Background(), CreateGroupInput{
		Name:        "engineering",
		DisplayName: "Engineering",
		ProviderIDs: []string{openai.ID.String()},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if len(group.ModelPatterns) != 1 || group.ModelPatterns[0] != defaultAllModelsPattern {
		t.Fatalf("expected default all-model pattern, got %#v", group.ModelPatterns)
	}
	if len(group.ExcludedModelPatterns) != 0 {
		t.Fatalf("expected empty excluded model patterns, got %#v", group.ExcludedModelPatterns)
	}
}

// TestUpdateGroupIgnoresName verifies group technical names are immutable through updates.
func TestUpdateGroupIgnoresName(t *testing.T) {
	service, _, _, openai, _ := newGroupTestService(t)
	ctx := context.Background()
	group, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:        "engineering",
		DisplayName: "Engineering",
		ProviderIDs: []string{openai.ID.String()},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	nextName := "platform"
	nextDisplayName := "Platform"
	updated, err := service.UpdateGroup(ctx, group.ID.String(), UpdateGroupInput{
		Name:        &nextName,
		DisplayName: &nextDisplayName,
	})
	if err != nil {
		t.Fatalf("update group: %v", err)
	}
	if updated.Name != group.Name {
		t.Fatalf("expected group name to remain %q, got %q", group.Name, updated.Name)
	}
	if updated.DisplayName != nextDisplayName {
		t.Fatalf("expected display name %q, got %q", nextDisplayName, updated.DisplayName)
	}
}

// TestGroupMutationsNotifyDomainGroups verifies group mutations notify domain groups.
func TestGroupMutationsNotifyDomainGroups(t *testing.T) {
	service, _, user, openai, _ := newGroupTestService(t)
	ctx := context.Background()
	notifier := &recordingNotifier{}
	service.SetNotifier(notifier)

	group, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:        "engineering",
		DisplayName: "Engineering",
		ProviderIDs: []string{openai.ID.String()},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	name := group.Name
	providerIDs := []string{openai.ID.String()}
	if _, err := service.UpdateGroup(ctx, group.ID.String(), UpdateGroupInput{
		Name:        &name,
		ProviderIDs: &providerIDs,
	}); err != nil {
		t.Fatalf("update group: %v", err)
	}
	if err := service.AddMember(ctx, group.ID.String(), user.ID); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if err := service.RemoveMember(ctx, group.ID.String(), user.ID); err != nil {
		t.Fatalf("remove member: %v", err)
	}
	if _, err := service.ReplaceUserGroups(ctx, user.ID, []string{group.ID.String()}); err != nil {
		t.Fatalf("replace user groups: %v", err)
	}
	if err := service.DeleteGroup(ctx, group.ID.String()); err != nil {
		t.Fatalf("delete group: %v", err)
	}

	if len(notifier.domains) != 6 {
		t.Fatalf("expected 6 notifications, got %d: %#v", len(notifier.domains), notifier.domains)
	}
	for _, domain := range notifier.domains {
		if domain != configevents.DomainGroups {
			t.Fatalf("expected groups domain notification, got %q", domain)
		}
	}
}
