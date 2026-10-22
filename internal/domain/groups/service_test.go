package groups

import (
	"context"
	"errors"
	"reflect"
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

type groupUsageKPIFixture struct {
	Day                   time.Time `gorm:"type:date;not null;uniqueIndex:idx_group_usage_kpi_fixture"`
	InitiatorID           string    `gorm:"type:uuid;not null;uniqueIndex:idx_group_usage_kpi_fixture"`
	InputTokens           int64     `gorm:"not null;default:0"`
	OutputTokens          int64     `gorm:"not null;default:0"`
	CacheReadInputTokens  int64     `gorm:"not null;default:0"`
	CacheWriteInputTokens int64     `gorm:"not null;default:0"`
}

// TableName uses the production durable usage table without importing the
// proxy domain, which depends on runtime group snapshots.
func (groupUsageKPIFixture) TableName() string {
	return "proxy_daily_usage_kpis"
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
	if err := db.AutoMigrate(&users.User{}, &provider.Provider{}, &groupUsageKPIFixture{}); err != nil {
		t.Fatalf("migrate dependencies: %v", err)
	}
	service := NewService(db)
	if err := service.AutoMigrate(context.Background()); err != nil {
		t.Fatalf("migrate groups: %v", err)
	}

	user := users.User{
		ID:                uuid.NewString(),
		ExternalSub:       testPointer("oidc-sub"),
		Email:             "user@example.com",
		PreferredUsername: "user",
		Name:              "User",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       testPointer(time.Now().UTC()),
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

// createGroupUsageKPI stores durable daily token totals for one identity.
func createGroupUsageKPI(
	t *testing.T,
	db *gorm.DB,
	initiatorID string,
	day time.Time,
	inputTokens int64,
	outputTokens int64,
	cacheReadInputTokens int64,
	cacheWriteInputTokens int64,
) {
	t.Helper()
	row := groupUsageKPIFixture{
		Day:                   day,
		InitiatorID:           initiatorID,
		InputTokens:           inputTokens,
		OutputTokens:          outputTokens,
		CacheReadInputTokens:  cacheReadInputTokens,
		CacheWriteInputTokens: cacheWriteInputTokens,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create durable usage KPI: %v", err)
	}
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
	if got := store.GroupNames(user.ID); !reflect.DeepEqual(got, []string{"engineering"}) {
		t.Fatalf("unexpected technical group names: %#v", got)
	}
	if got := store.GroupNames(uuid.NewString()); got == nil || len(got) != 0 {
		t.Fatalf("expected an empty group list for an unknown user, got %#v", got)
	}
}

// TestSnapshotIncludesGroupsWithoutActiveProviders verifies OPA membership data is independent from access grants.
func TestSnapshotIncludesGroupsWithoutActiveProviders(t *testing.T) {
	service, db, user, openai, _ := newGroupTestService(t)
	ctx := context.Background()
	group, err := service.CreateGroup(ctx, CreateGroupInput{
		Name: "dormant", DisplayName: "Dormant", ProviderIDs: []string{openai.ID.String()},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := service.ReplaceUserGroups(ctx, user.ID, []string{group.ID.String()}); err != nil {
		t.Fatalf("replace user groups: %v", err)
	}
	if err := db.Model(&provider.Provider{}).Where("id = ?", openai.ID).Update("enabled", false).Error; err != nil {
		t.Fatalf("disable provider: %v", err)
	}

	store := NewSnapshotStore(service)
	if err := store.Refresh(ctx); err != nil {
		t.Fatalf("refresh snapshot: %v", err)
	}
	if got := store.GroupNames(user.ID); !reflect.DeepEqual(got, []string{"dormant"}) {
		t.Fatalf("expected group without active provider, got %#v", got)
	}
	if store.AllowsProvider(user.ID, openai.Name) {
		t.Fatal("disabled provider must remain inaccessible")
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
		ExternalSub:       testPointer("service-sub"),
		PreferredUsername: "automation",
		Name:              "Automation",
		Type:              auth.UserTypeService,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       testPointer(time.Now().UTC()),
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

// TestListGroupsPagedIncludesCurrentMemberUsageTotals verifies list totals use
// durable all-time KPIs for the current human and service-account membership.
func TestListGroupsPagedIncludesCurrentMemberUsageTotals(t *testing.T) {
	service, db, human, openai, _ := newGroupTestService(t)
	ctx := context.Background()
	serviceAccount := users.User{
		ID:                uuid.NewString(),
		ExternalSub:       testPointer("service-usage-sub"),
		PreferredUsername: "usage-automation",
		Name:              "Usage Automation",
		Type:              auth.UserTypeService,
		Role:              auth.RoleUser,
		IsActive:          false,
		LastLoginAt:       testPointer(time.Now().UTC()),
	}
	outsider := users.User{
		ID:                uuid.NewString(),
		ExternalSub:       testPointer("outsider-usage-sub"),
		Email:             "outsider-usage@example.com",
		PreferredUsername: "outsider-usage",
		Name:              "Outsider Usage",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleUser,
		IsActive:          true,
		LastLoginAt:       testPointer(time.Now().UTC()),
	}
	if err := db.Create(&serviceAccount).Error; err != nil {
		t.Fatalf("create inactive service account: %v", err)
	}
	if err := db.Create(&outsider).Error; err != nil {
		t.Fatalf("create outsider: %v", err)
	}

	primary, err := service.CreateGroup(ctx, CreateGroupInput{
		Name: "primary-usage", DisplayName: "Primary Usage", ProviderIDs: []string{openai.ID.String()},
	})
	if err != nil {
		t.Fatalf("create primary group: %v", err)
	}
	overlap, err := service.CreateGroup(ctx, CreateGroupInput{
		Name: "overlap-usage", DisplayName: "Overlap Usage", ProviderIDs: []string{openai.ID.String()},
	})
	if err != nil {
		t.Fatalf("create overlap group: %v", err)
	}
	empty, err := service.CreateGroup(ctx, CreateGroupInput{
		Name: "empty-usage", DisplayName: "Empty Usage", ProviderIDs: []string{openai.ID.String()},
	})
	if err != nil {
		t.Fatalf("create empty group: %v", err)
	}

	// Usage predates membership to assert that list totals follow the current
	// cohort rather than membership creation timestamps.
	day := time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC)
	createGroupUsageKPI(t, db, human.ID, day, 10, 4, 100, 200)
	createGroupUsageKPI(t, db, human.ID, day.AddDate(0, 0, 1), 5, 2, 300, 400)
	createGroupUsageKPI(t, db, serviceAccount.ID, day, 7, 3, 500, 600)
	createGroupUsageKPI(t, db, outsider.ID, day, 1_000, 2_000, 0, 0)

	if _, err := service.ReplaceUserGroups(ctx, human.ID, []string{primary.ID.String(), overlap.ID.String()}); err != nil {
		t.Fatalf("assign human groups: %v", err)
	}
	if _, err := service.ReplaceUserGroups(ctx, serviceAccount.ID, []string{primary.ID.String()}); err != nil {
		t.Fatalf("assign service-account group: %v", err)
	}

	result, err := service.ListGroupsPaged(ctx, ListParams{Page: 1, PageSize: 10, SortBy: "name", SortDir: "asc"})
	if err != nil {
		t.Fatalf("list groups with usage totals: %v", err)
	}
	items := make(map[uuid.UUID]GroupListItemResponse, len(result.Items))
	for _, item := range result.Items {
		items[item.ID] = item
	}
	if got := items[primary.ID]; got.InputTokens != 22 || got.OutputTokens != 9 {
		t.Fatalf("expected human and inactive service totals 22/9 without cache or outsider usage, got %#v", got)
	}
	if got := items[overlap.ID]; got.InputTokens != 15 || got.OutputTokens != 6 {
		t.Fatalf("expected overlapping human totals 15/6, got %#v", got)
	}
	if got := items[empty.ID]; got.InputTokens != 0 || got.OutputTokens != 0 {
		t.Fatalf("expected empty group totals 0/0, got %#v", got)
	}

	if _, err := service.ReplaceUserGroups(ctx, serviceAccount.ID, nil); err != nil {
		t.Fatalf("remove service-account group: %v", err)
	}
	result, err = service.ListGroupsPaged(ctx, ListParams{Page: 1, PageSize: 10, SortBy: "name", SortDir: "asc"})
	if err != nil {
		t.Fatalf("list groups after member removal: %v", err)
	}
	for _, item := range result.Items {
		if item.ID == primary.ID && (item.InputTokens != 15 || item.OutputTokens != 6) {
			t.Fatalf("expected removed member usage to be excluded, got %#v", item)
		}
	}
}

// TestListGroupsPagedSortsUsageBeforePagination verifies durable token sorting
// is performed by the database before offset and limit are applied.
func TestListGroupsPagedSortsUsageBeforePagination(t *testing.T) {
	service, db, firstUser, openai, _ := newGroupTestService(t)
	ctx := context.Background()
	usersByName := map[string]users.User{"low-usage": firstUser}
	for _, name := range []string{"medium-usage", "high-usage"} {
		user := users.User{
			ID:                uuid.NewString(),
			ExternalSub:       testPointer(name + "-sub"),
			Email:             name + "@example.com",
			PreferredUsername: name,
			Name:              name,
			Type:              auth.UserTypeUser,
			Role:              auth.RoleUser,
			IsActive:          true,
			LastLoginAt:       testPointer(time.Now().UTC()),
		}
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("create %s user: %v", name, err)
		}
		usersByName[name] = user
	}

	type usageFixture struct {
		name         string
		inputTokens  int64
		outputTokens int64
	}
	fixtures := []usageFixture{
		{name: "low-usage", inputTokens: 10, outputTokens: 90},
		{name: "medium-usage", inputTokens: 20, outputTokens: 50},
		{name: "high-usage", inputTokens: 30, outputTokens: 10},
	}
	groupsByName := make(map[string]GroupResponse, len(fixtures))
	day := time.Date(2026, time.October, 21, 0, 0, 0, 0, time.UTC)
	for _, fixture := range fixtures {
		group, err := service.CreateGroup(ctx, CreateGroupInput{
			Name: fixture.name, DisplayName: fixture.name, ProviderIDs: []string{openai.ID.String()},
		})
		if err != nil {
			t.Fatalf("create %s group: %v", fixture.name, err)
		}
		groupsByName[fixture.name] = group
		member := usersByName[fixture.name]
		if _, err := service.ReplaceUserGroups(ctx, member.ID, []string{group.ID.String()}); err != nil {
			t.Fatalf("assign %s group: %v", fixture.name, err)
		}
		createGroupUsageKPI(t, db, member.ID, day, fixture.inputTokens, fixture.outputTokens, 0, 0)
	}

	result, err := service.ListGroupsPaged(ctx, ListParams{
		Page: 2, PageSize: 1, SortBy: "inputTokens", SortDir: "desc",
	})
	if err != nil {
		t.Fatalf("sort input tokens before pagination: %v", err)
	}
	if result.Total != 3 || len(result.Items) != 1 || result.Items[0].ID != groupsByName["medium-usage"].ID {
		t.Fatalf("unexpected paginated input-token order: %#v", result)
	}
	if result.Items[0].InputTokens != 20 || result.Items[0].OutputTokens != 50 {
		t.Fatalf("unexpected medium group totals: %#v", result.Items[0])
	}

	result, err = service.ListGroupsPaged(ctx, ListParams{
		Page: 1, PageSize: 2, SortBy: "outputTokens", SortDir: "asc",
	})
	if err != nil {
		t.Fatalf("sort output tokens before pagination: %v", err)
	}
	if len(result.Items) != 2 || result.Items[0].ID != groupsByName["high-usage"].ID || result.Items[1].ID != groupsByName["medium-usage"].ID {
		t.Fatalf("unexpected output-token order: %#v", result.Items)
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

func TestPendingUserMembershipSurvivesFirstSSOSync(t *testing.T) {
	service, db, _, openai, _ := newGroupTestService(t)
	ctx := context.Background()
	pending := users.User{
		ID:                uuid.NewString(),
		PreferredUsername: "pending-group-user",
		Type:              auth.UserTypeUser,
		Role:              auth.RoleLimited,
		IsActive:          true,
	}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatalf("create pending group user: %v", err)
	}
	group, err := service.CreateGroup(ctx, CreateGroupInput{
		Name:        "preassigned-access",
		DisplayName: "Preassigned Access",
		ProviderIDs: []string{openai.ID.String()},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := service.AddMember(ctx, group.ID.String(), pending.ID); err != nil {
		t.Fatalf("assign pending user group: %v", err)
	}

	profile, err := users.NewService(db).SyncUser(ctx, auth.Identity{
		Sub:               "pending-group-subject",
		PreferredUsername: "pending-group-user",
		Email:             "pending-group@example.com",
		Name:              "Pending Group User",
	})
	if err != nil {
		t.Fatalf("claim pending group user: %v", err)
	}
	if profile.ID != pending.ID || profile.Role != auth.RoleLimited {
		t.Fatalf("expected stable local user and role, got %#v", profile)
	}
	assigned, err := service.ListUserGroups(ctx, pending.ID)
	if err != nil {
		t.Fatalf("list claimed user groups: %v", err)
	}
	if len(assigned) != 1 || assigned[0].ID != group.ID {
		t.Fatalf("expected preassigned group to survive SSO claim, got %#v", assigned)
	}
}
