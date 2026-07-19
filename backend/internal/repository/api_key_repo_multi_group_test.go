package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestNormalizeAPIKeyGroupIDs(t *testing.T) {
	defaultGroupID := int64(12)

	groupIDs, err := normalizedAPIKeyGroupIDs(&defaultGroupID, []int64{12, 2, 12, 11})
	require.NoError(t, err)
	require.Equal(t, []int64{2, 11, 12}, groupIDs)

	groupIDs, err = normalizedAPIKeyGroupIDs(&defaultGroupID, nil)
	require.NoError(t, err)
	require.Equal(t, []int64{12}, groupIDs)

	groupIDs, err = normalizedAPIKeyGroupIDs(nil, nil)
	require.NoError(t, err)
	require.Empty(t, groupIDs)

	_, err = normalizedAPIKeyGroupIDs(&defaultGroupID, []int64{2, 11})
	require.ErrorIs(t, err, service.ErrAPIKeyGroupNotBound)

	_, err = normalizedAPIKeyGroupIDs(nil, []int64{2, 11})
	require.ErrorIs(t, err, service.ErrAPIKeyGroupNotBound)
}

func TestAPIKeyRepositoryCreateAndAuthReadMultipleGroups(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "multi-group-create@test.com")
	openAIGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Codex", service.PlatformOpenAI)
	grokGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Grok", service.PlatformGrok)

	key := &service.APIKey{
		UserID:   user.ID,
		Key:      "sk-multi-group-create",
		Name:     "Multi Group",
		GroupID:  &openAIGroup.ID,
		GroupIDs: []int64{grokGroup.ID, openAIGroup.ID},
		Status:   service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	got, err := repo.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.Equal(t, []int64{openAIGroup.ID, grokGroup.ID}, got.GroupIDs)
	require.Len(t, got.Groups, 2)
	require.Equal(t, service.PlatformOpenAI, got.Groups[0].Platform)
	require.Equal(t, service.PlatformGrok, got.Groups[1].Platform)
	require.Equal(t, openAIGroup.ID, *got.GroupID)
}

func TestAPIKeyRepositoryLegacyCreateBackfillsOneBinding(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "legacy-group-create@test.com")
	group := mustCreateAPIKeyRepoGroup(t, ctx, client, "Legacy", service.PlatformOpenAI)

	key := &service.APIKey{
		UserID:  user.ID,
		Key:     "sk-legacy-group-create",
		Name:    "Legacy Group",
		GroupID: &group.ID,
		Status:  service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	got, err := repo.GetByID(ctx, key.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{group.ID}, got.GroupIDs)
	require.Len(t, got.Groups, 1)
}

func TestAPIKeyRepositoryUpdateReplacesBindings(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "multi-group-update@test.com")
	openAIGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Codex Update", service.PlatformOpenAI)
	claudeGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Claude Update", service.PlatformAnthropic)
	grokGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Grok Update", service.PlatformGrok)

	key := &service.APIKey{
		UserID:   user.ID,
		Key:      "sk-multi-group-update",
		Name:     "Multi Group Update",
		GroupID:  &openAIGroup.ID,
		GroupIDs: []int64{openAIGroup.ID, grokGroup.ID},
		Status:   service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	key.GroupID = &claudeGroup.ID
	key.GroupIDs = []int64{grokGroup.ID, claudeGroup.ID}
	require.NoError(t, repo.Update(ctx, key))

	got, err := repo.GetByID(ctx, key.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{claudeGroup.ID, grokGroup.ID}, got.GroupIDs)
	require.Equal(t, claudeGroup.ID, *got.GroupID)
}

func TestAPIKeyRepositoryLegacyUpdateReplacesBindingsWithDefaultOnly(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "legacy-group-update@test.com")
	openAIGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Legacy Update Codex", service.PlatformOpenAI)
	grokGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Legacy Update Grok", service.PlatformGrok)

	key := &service.APIKey{
		UserID:   user.ID,
		Key:      "sk-legacy-group-update",
		Name:     "Legacy Group Update",
		GroupID:  &openAIGroup.ID,
		GroupIDs: []int64{openAIGroup.ID, grokGroup.ID},
		Status:   service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	key.GroupID = &grokGroup.ID
	key.GroupIDs = nil
	require.NoError(t, repo.Update(ctx, key))

	got, err := repo.GetByID(ctx, key.ID)
	require.NoError(t, err)
	require.Equal(t, grokGroup.ID, *got.GroupID)
	require.Equal(t, []int64{grokGroup.ID}, got.GroupIDs)
}

func TestAPIKeyRepositoryUpdateAllowsEmptyBindings(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "empty-group-update@test.com")
	group := mustCreateAPIKeyRepoGroup(t, ctx, client, "Empty Group Update", service.PlatformOpenAI)

	key := &service.APIKey{
		UserID:  user.ID,
		Key:     "sk-empty-group-update",
		Name:    "Empty Group Update",
		GroupID: &group.ID,
		Status:  service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	key.GroupID = nil
	key.GroupIDs = nil
	require.NoError(t, repo.Update(ctx, key))

	got, err := repo.GetByID(ctx, key.ID)
	require.NoError(t, err)
	require.Nil(t, got.GroupID)
	require.Empty(t, got.GroupIDs)
	require.Empty(t, got.Groups)
}

func TestAPIKeyRepositoryCreateRollsBackWhenBindingWriteFails(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "multi-group-create-rollback@test.com")
	group := mustCreateAPIKeyRepoGroup(t, ctx, client, "Create Rollback", service.PlatformOpenAI)

	key := &service.APIKey{
		UserID:   user.ID,
		Key:      "sk-multi-group-create-rollback",
		Name:     "Create Rollback",
		GroupID:  &group.ID,
		GroupIDs: []int64{group.ID, group.ID + 1_000_000},
		Status:   service.StatusActive,
	}
	require.Error(t, repo.Create(ctx, key))

	exists, err := client.APIKey.Query().Where(apikey.KeyEQ(key.Key)).Exist(ctx)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestAPIKeyRepositoryUpdateRollsBackWhenBindingWriteFails(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "multi-group-update-rollback@test.com")
	originalGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Original Rollback", service.PlatformOpenAI)
	newDefaultGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "New Rollback", service.PlatformGrok)

	key := &service.APIKey{
		UserID:  user.ID,
		Key:     "sk-multi-group-update-rollback",
		Name:    "Before Rollback",
		GroupID: &originalGroup.ID,
		Status:  service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	key.Name = "Must Roll Back"
	key.GroupID = &newDefaultGroup.ID
	key.GroupIDs = []int64{newDefaultGroup.ID, newDefaultGroup.ID + 1_000_000}
	require.Error(t, repo.Update(ctx, key))

	got, err := repo.GetByID(ctx, key.ID)
	require.NoError(t, err)
	require.Equal(t, "Before Rollback", got.Name)
	require.Equal(t, originalGroup.ID, *got.GroupID)
	require.Equal(t, []int64{originalGroup.ID}, got.GroupIDs)
}

func TestAPIKeyRepositoryReadFallsBackToLegacyDefaultGroup(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "legacy-group-read@test.com")
	group := mustCreateAPIKeyRepoGroup(t, ctx, client, "Legacy Read", service.PlatformOpenAI)

	created, err := client.APIKey.Create().
		SetUserID(user.ID).
		SetKey("sk-legacy-group-read").
		SetName("Legacy Read").
		SetGroupID(group.ID).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{group.ID}, got.GroupIDs)
	require.Len(t, got.Groups, 1)
	require.Equal(t, group.ID, got.Groups[0].ID)
}

func TestAPIKeyRepositoryReadMergesLegacyDefaultWithJoinBindings(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "rolling-upgrade-read@test.com")
	joinGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Rolling Join", service.PlatformOpenAI)
	legacyDefault := mustCreateAPIKeyRepoGroup(t, ctx, client, "Rolling Default", service.PlatformGrok)

	created, err := client.APIKey.Create().
		SetUserID(user.ID).
		SetKey("sk-rolling-upgrade-read").
		SetName("Rolling Upgrade Read").
		SetGroupID(legacyDefault.ID).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.APIKeyGroup.Create().
		SetAPIKeyID(created.ID).
		SetGroupID(joinGroup.ID).
		Save(ctx)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{joinGroup.ID, legacyDefault.ID}, got.GroupIDs)
	require.Equal(t, legacyDefault.ID, *got.GroupID)

	got.Name = "Rolling Upgrade Updated"
	require.NoError(t, repo.Update(ctx, got))
	afterUpdate, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{joinGroup.ID, legacyDefault.ID}, afterUpdate.GroupIDs)
}

func TestAPIKeyRepositoryListReadsMultipleGroups(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "multi-group-list@test.com")
	openAIGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Codex List", service.PlatformOpenAI)
	grokGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Grok List", service.PlatformGrok)

	key := &service.APIKey{
		UserID:   user.ID,
		Key:      "sk-multi-group-list",
		Name:     "Multi Group List",
		GroupID:  &openAIGroup.ID,
		GroupIDs: []int64{grokGroup.ID, openAIGroup.ID},
		Status:   service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	keys, _, err := repo.ListByUserID(ctx, user.ID, pagination.PaginationParams{Page: 1, PageSize: 10}, service.APIKeyListFilters{})
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, []int64{openAIGroup.ID, grokGroup.ID}, keys[0].GroupIDs)

	allKeys, err := repo.ListAllByUserID(ctx, user.ID, service.APIKeyListFilters{})
	require.NoError(t, err)
	require.Len(t, allKeys, 1)
	require.Equal(t, []int64{openAIGroup.ID, grokGroup.ID}, allKeys[0].GroupIDs)

	groupKeys, _, err := repo.ListByGroupID(ctx, openAIGroup.ID, pagination.PaginationParams{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, groupKeys, 1)
	require.Equal(t, []int64{openAIGroup.ID, grokGroup.ID}, groupKeys[0].GroupIDs)
}

func TestAPIKeyRepositoryGroupLookupsIncludeSecondaryBindings(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "secondary-group-lookups@test.com")
	defaultGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Secondary Lookup Default", service.PlatformOpenAI)
	secondaryGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Secondary Lookup Bound", service.PlatformGrok)

	key := &service.APIKey{
		UserID:   user.ID,
		Key:      "sk-secondary-group-lookups",
		Name:     "Secondary Group Lookups",
		GroupID:  &defaultGroup.ID,
		GroupIDs: []int64{defaultGroup.ID, secondaryGroup.ID},
		Status:   service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	keys, result, err := repo.ListByGroupID(ctx, secondaryGroup.ID, pagination.PaginationParams{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Len(t, keys, 1)
	require.Equal(t, key.ID, keys[0].ID)

	count, err := repo.CountByGroupID(ctx, secondaryGroup.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	keyValues, err := repo.ListKeysByGroupID(ctx, secondaryGroup.ID)
	require.NoError(t, err)
	require.Equal(t, []string{key.Key}, keyValues)
}

func TestAPIKeyRepositoryUserListGroupFiltersUseAllBindings(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "user-list-group-filter@test.com")
	defaultGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Filter Default", service.PlatformOpenAI)
	secondaryGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Filter Secondary", service.PlatformGrok)

	multiGroup := &service.APIKey{
		UserID:   user.ID,
		Key:      "sk-filter-secondary-binding",
		Name:     "Secondary Binding",
		GroupID:  &defaultGroup.ID,
		GroupIDs: []int64{defaultGroup.ID, secondaryGroup.ID},
		Status:   service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, multiGroup))

	joinOnly, err := client.APIKey.Create().
		SetUserID(user.ID).
		SetKey("sk-filter-join-only").
		SetName("Join Only").
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.APIKeyGroup.Create().
		SetAPIKeyID(joinOnly.ID).
		SetGroupID(secondaryGroup.ID).
		Save(ctx)
	require.NoError(t, err)

	unbound := &service.APIKey{
		UserID: user.ID,
		Key:    "sk-filter-unbound",
		Name:   "Unbound",
		Status: service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, unbound))

	assertFilter := func(groupID int64, expectedIDs []int64) {
		t.Helper()
		filters := service.APIKeyListFilters{GroupID: &groupID}
		paged, result, err := repo.ListByUserID(ctx, user.ID, pagination.PaginationParams{Page: 1, PageSize: 10}, filters)
		require.NoError(t, err)
		require.Equal(t, int64(len(expectedIDs)), result.Total)
		pagedIDs := make([]int64, 0, len(paged))
		for i := range paged {
			pagedIDs = append(pagedIDs, paged[i].ID)
		}
		require.ElementsMatch(t, expectedIDs, pagedIDs)

		all, err := repo.ListAllByUserID(ctx, user.ID, filters)
		require.NoError(t, err)
		allIDs := make([]int64, 0, len(all))
		for i := range all {
			allIDs = append(allIDs, all[i].ID)
		}
		require.ElementsMatch(t, expectedIDs, allIDs)
	}

	assertFilter(secondaryGroup.ID, []int64{multiGroup.ID, joinOnly.ID})
	assertFilter(0, []int64{unbound.ID})
}

func TestAPIKeyRepositoryUpdateGroupIDByUserAndGroupMigratesAnyBinding(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "migrate-any-binding@test.com")
	otherUser := mustCreateAPIKeyRepoUser(t, ctx, client, "migrate-any-binding-other@test.com")
	oldGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Migrate Old", service.PlatformOpenAI)
	newGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Migrate New", service.PlatformGrok)
	otherGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Migrate Other", service.PlatformAnthropic)

	defaultOld := &service.APIKey{
		UserID:   user.ID,
		Key:      "sk-migrate-default-old",
		Name:     "Migrate Default Old",
		GroupID:  &oldGroup.ID,
		GroupIDs: []int64{oldGroup.ID, otherGroup.ID},
		Status:   service.StatusActive,
	}
	secondaryOld := &service.APIKey{
		UserID:   user.ID,
		Key:      "sk-migrate-secondary-old",
		Name:     "Migrate Secondary Old",
		GroupID:  &otherGroup.ID,
		GroupIDs: []int64{oldGroup.ID, newGroup.ID, otherGroup.ID},
		Status:   service.StatusActive,
	}
	otherUserKey := &service.APIKey{
		UserID:   otherUser.ID,
		Key:      "sk-migrate-other-user",
		Name:     "Migrate Other User",
		GroupID:  &oldGroup.ID,
		GroupIDs: []int64{oldGroup.ID, otherGroup.ID},
		Status:   service.StatusActive,
	}
	for _, key := range []*service.APIKey{defaultOld, secondaryOld, otherUserKey} {
		require.NoError(t, repo.Create(ctx, key))
	}

	affected, err := repo.UpdateGroupIDByUserAndGroup(ctx, user.ID, oldGroup.ID, newGroup.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), affected)

	gotDefaultOld, err := repo.GetByID(ctx, defaultOld.ID)
	require.NoError(t, err)
	require.Equal(t, newGroup.ID, *gotDefaultOld.GroupID)
	require.Equal(t, []int64{newGroup.ID, otherGroup.ID}, gotDefaultOld.GroupIDs)

	gotSecondaryOld, err := repo.GetByID(ctx, secondaryOld.ID)
	require.NoError(t, err)
	require.Equal(t, otherGroup.ID, *gotSecondaryOld.GroupID)
	require.Equal(t, []int64{newGroup.ID, otherGroup.ID}, gotSecondaryOld.GroupIDs)

	gotOtherUser, err := repo.GetByID(ctx, otherUserKey.ID)
	require.NoError(t, err)
	require.Equal(t, oldGroup.ID, *gotOtherUser.GroupID)
	require.Equal(t, []int64{oldGroup.ID, otherGroup.ID}, gotOtherUser.GroupIDs)
}

func TestAPIKeyRepositoryClearGroupIDByGroupIDRepairsDefaults(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "clear-group-bindings@test.com")
	group2 := mustCreateAPIKeyRepoGroup(t, ctx, client, "Clear Group 2", service.PlatformOpenAI)
	group11 := mustCreateAPIKeyRepoGroup(t, ctx, client, "Clear Group 11", service.PlatformAnthropic)
	group12 := mustCreateAPIKeyRepoGroup(t, ctx, client, "Clear Group 12", service.PlatformGrok)

	defaultRemoved := &service.APIKey{
		UserID:   user.ID,
		Key:      "sk-clear-default",
		Name:     "Clear Default",
		GroupID:  &group12.ID,
		GroupIDs: []int64{group12.ID, group11.ID, group2.ID},
		Status:   service.StatusActive,
	}
	secondaryRemoved := &service.APIKey{
		UserID:   user.ID,
		Key:      "sk-clear-secondary",
		Name:     "Clear Secondary",
		GroupID:  &group2.ID,
		GroupIDs: []int64{group2.ID, group12.ID},
		Status:   service.StatusActive,
	}
	lastBindingRemoved := &service.APIKey{
		UserID:  user.ID,
		Key:     "sk-clear-last",
		Name:    "Clear Last",
		GroupID: &group12.ID,
		Status:  service.StatusActive,
	}
	for _, key := range []*service.APIKey{defaultRemoved, secondaryRemoved, lastBindingRemoved} {
		require.NoError(t, repo.Create(ctx, key))
	}

	affected, err := repo.ClearGroupIDByGroupID(ctx, group12.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), affected)

	gotDefaultRemoved, err := repo.GetByID(ctx, defaultRemoved.ID)
	require.NoError(t, err)
	require.Equal(t, group2.ID, *gotDefaultRemoved.GroupID)
	require.Equal(t, []int64{group2.ID, group11.ID}, gotDefaultRemoved.GroupIDs)

	gotSecondaryRemoved, err := repo.GetByID(ctx, secondaryRemoved.ID)
	require.NoError(t, err)
	require.Equal(t, group2.ID, *gotSecondaryRemoved.GroupID)
	require.Equal(t, []int64{group2.ID}, gotSecondaryRemoved.GroupIDs)

	gotLastBindingRemoved, err := repo.GetByID(ctx, lastBindingRemoved.ID)
	require.NoError(t, err)
	require.Nil(t, gotLastBindingRemoved.GroupID)
	require.Empty(t, gotLastBindingRemoved.GroupIDs)
}

func TestAPIKeyRepositoryOuterTxContextLeavesRollbackToCaller(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "outer-tx-context@test.com")
	defaultGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Outer Tx Default", service.PlatformOpenAI)
	secondaryGroup := mustCreateAPIKeyRepoGroup(t, ctx, client, "Outer Tx Secondary", service.PlatformGrok)

	key := &service.APIKey{
		UserID:   user.ID,
		Key:      "sk-outer-tx-context",
		Name:     "Outer Tx Context",
		GroupID:  &defaultGroup.ID,
		GroupIDs: []int64{defaultGroup.ID, secondaryGroup.ID},
		Status:   service.StatusActive,
	}
	require.NoError(t, repo.Create(ctx, key))

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx := dbent.NewTxContext(ctx, tx)
	invalidGroupID := secondaryGroup.ID + 1_000_000

	_, err = repo.UpdateGroupIDByUserAndGroup(txCtx, user.ID, secondaryGroup.ID, invalidGroupID)
	require.Error(t, err)
	require.NoError(t, tx.Rollback(), "repository must leave the caller-owned transaction open")

	got, err := repo.GetByID(ctx, key.ID)
	require.NoError(t, err)
	require.Equal(t, defaultGroup.ID, *got.GroupID)
	require.Equal(t, []int64{defaultGroup.ID, secondaryGroup.ID}, got.GroupIDs)
}

func TestAPIKeyEntityToServiceSortsAndDeduplicatesGroups(t *testing.T) {
	defaultGroupID := int64(12)
	defaultGroup := &dbent.Group{ID: 12, Name: "Grok"}
	entity := &dbent.APIKey{
		GroupID: &defaultGroupID,
		Edges: dbent.APIKeyEdges{
			Group: defaultGroup,
			Groups: []*dbent.Group{
				defaultGroup,
				{ID: 2, Name: "Codex"},
				{ID: 12, Name: "Grok Duplicate"},
			},
		},
	}

	got := apiKeyEntityToService(entity)
	require.Equal(t, []int64{2, 12}, got.GroupIDs)
	require.Len(t, got.Groups, 2)
	require.Equal(t, int64(2), got.Groups[0].ID)
	require.Equal(t, int64(12), got.Groups[1].ID)
}

func TestAPIKeyRepositoryBulkBindingQueriesLockPostgresRowsInIDOrder(t *testing.T) {
	tests := []struct {
		name string
		call func(*apiKeyRepository) (int64, error)
	}{
		{
			name: "clear",
			call: func(repo *apiKeyRepository) (int64, error) {
				return repo.ClearGroupIDByGroupID(context.Background(), 12)
			},
		},
		{
			name: "migrate",
			call: func(repo *apiKeyRepository) (int64, error) {
				return repo.UpdateGroupIDByUserAndGroup(context.Background(), 7, 12, 13)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			t.Cleanup(func() { _ = client.Close() })
			repo := newAPIKeyRepositoryWithSQL(client, db)

			mock.ExpectBegin()
			mock.ExpectQuery(`(?s)` + regexp.QuoteMeta(`SELECT`) + `.*` + regexp.QuoteMeta(`FROM "api_keys"`) + `.*ORDER BY .*"id".*FOR UPDATE`).
				WillReturnRows(sqlmock.NewRows([]string{"id", "group_id"}))
			mock.ExpectCommit()

			affected, err := tt.call(repo)
			require.NoError(t, err)
			require.Zero(t, affected)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func mustCreateAPIKeyRepoGroup(t *testing.T, ctx context.Context, client *dbent.Client, name, platform string) *service.Group {
	t.Helper()
	group, err := client.Group.Create().
		SetName(name).
		SetPlatform(platform).
		SetStatus(service.StatusActive).
		SetSubscriptionType(service.SubscriptionTypeStandard).
		SetRateMultiplier(1).
		Save(ctx)
	require.NoError(t, err)
	return groupEntityToService(group)
}
