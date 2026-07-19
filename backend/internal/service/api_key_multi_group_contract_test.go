package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyServiceCreatePersistsNormalizedGroupBindings(t *testing.T) {
	repo := &multiGroupContractAPIKeyRepo{}
	groups := multiGroupContractGroups()
	service := newMultiGroupContractAPIKeyService(repo, groups)
	defaultID := int64(2)

	created, err := service.Create(context.Background(), 7, CreateAPIKeyRequest{
		Name:     "CC Switch",
		GroupID:  &defaultID,
		GroupIDs: []int64{12, 2, 11, 12},
	})

	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, []int64{2, 11, 12}, repo.created.GroupIDs)
	require.Equal(t, []int64{2, 11, 12}, created.GroupIDs)
	require.Equal(t, defaultID, created.Group.ID)
	require.Equal(t, []int64{2, 11, 12}, groupIDsFromGroups(created.Groups))
}

func TestAPIKeyServiceCreateRejectsInvalidMultiGroupContracts(t *testing.T) {
	defaultID := int64(2)
	tests := []struct {
		name      string
		defaultID *int64
		groupIDs  []int64
		groups    map[int64]Group
	}{
		{name: "explicit empty bindings", defaultID: &defaultID, groupIDs: []int64{}, groups: multiGroupContractGroups()},
		{name: "missing default", defaultID: &defaultID, groupIDs: []int64{11, 12}, groups: multiGroupContractGroups()},
		{name: "bindings without default", defaultID: nil, groupIDs: []int64{2, 11}, groups: multiGroupContractGroups()},
		{name: "nonpositive binding", defaultID: &defaultID, groupIDs: []int64{0, 2}, groups: multiGroupContractGroups()},
		{name: "missing binding", defaultID: &defaultID, groupIDs: []int64{2, 99}, groups: multiGroupContractGroups()},
		{name: "inaccessible binding", defaultID: &defaultID, groupIDs: []int64{2, 11}, groups: map[int64]Group{
			2:  {ID: 2, Platform: PlatformOpenAI, Status: StatusActive},
			11: {ID: 11, Platform: PlatformAnthropic, Status: StatusActive, IsExclusive: true},
		}},
		{name: "inactive binding", defaultID: &defaultID, groupIDs: []int64{2, 11}, groups: map[int64]Group{
			2:  {ID: 2, Platform: PlatformOpenAI, Status: StatusActive},
			11: {ID: 11, Platform: PlatformAnthropic, Status: StatusDisabled},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &multiGroupContractAPIKeyRepo{}
			service := newMultiGroupContractAPIKeyService(repo, tt.groups)

			created, err := service.Create(context.Background(), 7, CreateAPIKeyRequest{
				Name:     "invalid",
				GroupID:  tt.defaultID,
				GroupIDs: tt.groupIDs,
			})

			require.Nil(t, created)
			require.Error(t, err)
			require.Nil(t, repo.created)
		})
	}
}

func TestAPIKeyServiceCreateRejectsExcessiveGroupBindings(t *testing.T) {
	groupIDs := make([]int64, 1025)
	for i := range groupIDs {
		groupIDs[i] = int64(i + 1)
	}
	defaultID := int64(1)
	repo := &multiGroupContractAPIKeyRepo{}
	service := newMultiGroupContractAPIKeyService(repo, multiGroupContractGroups())

	created, err := service.Create(context.Background(), 7, CreateAPIKeyRequest{
		Name:     "too many bindings",
		GroupID:  &defaultID,
		GroupIDs: groupIDs,
	})

	require.Nil(t, created)
	require.ErrorIs(t, err, ErrAPIKeyTooManyGroups)
	require.Nil(t, repo.created)
}

func TestAPIKeyServiceUpdatePreservesOrReplacesGroupBindingsByContract(t *testing.T) {
	defaultID := int64(2)
	base := &APIKey{
		ID:       100,
		UserID:   7,
		Key:      "sk-existing-multi-group",
		Name:     "Existing",
		Status:   StatusActive,
		GroupID:  &defaultID,
		Group:    cloneAPIKeyGroupPointer(multiGroupContractGroups()[2]),
		GroupIDs: []int64{2, 11, 12},
		Groups:   groupsFromMap(multiGroupContractGroups(), []int64{2, 11, 12}),
	}

	t.Run("omitted group fields preserve bindings", func(t *testing.T) {
		repo := &multiGroupContractAPIKeyRepo{existing: cloneMultiGroupContractAPIKey(base)}
		service := newMultiGroupContractAPIKeyService(repo, multiGroupContractGroups())
		name := "Renamed"

		updated, err := service.Update(context.Background(), base.ID, base.UserID, UpdateAPIKeyRequest{Name: &name})

		require.NoError(t, err)
		require.Equal(t, []int64{2, 11, 12}, updated.GroupIDs)
		require.Equal(t, defaultID, *updated.GroupID)
	})

	t.Run("legacy group id alone replaces with one binding", func(t *testing.T) {
		repo := &multiGroupContractAPIKeyRepo{existing: cloneMultiGroupContractAPIKey(base)}
		service := newMultiGroupContractAPIKeyService(repo, multiGroupContractGroups())
		newDefault := int64(11)

		updated, err := service.Update(context.Background(), base.ID, base.UserID, UpdateAPIKeyRequest{GroupID: &newDefault})

		require.NoError(t, err)
		require.Equal(t, []int64{11}, updated.GroupIDs)
		require.Equal(t, newDefault, *updated.GroupID)
	})

	t.Run("group ids can update while keeping current default", func(t *testing.T) {
		repo := &multiGroupContractAPIKeyRepo{existing: cloneMultiGroupContractAPIKey(base)}
		service := newMultiGroupContractAPIKeyService(repo, multiGroupContractGroups())

		updated, err := service.Update(context.Background(), base.ID, base.UserID, UpdateAPIKeyRequest{GroupIDs: []int64{12, 2, 12}})

		require.NoError(t, err)
		require.Equal(t, []int64{2, 12}, updated.GroupIDs)
		require.Equal(t, defaultID, *updated.GroupID)
	})

	t.Run("explicit empty bindings are rejected without update", func(t *testing.T) {
		repo := &multiGroupContractAPIKeyRepo{existing: cloneMultiGroupContractAPIKey(base)}
		service := newMultiGroupContractAPIKeyService(repo, multiGroupContractGroups())

		updated, err := service.Update(context.Background(), base.ID, base.UserID, UpdateAPIKeyRequest{GroupIDs: []int64{}})

		require.Nil(t, updated)
		require.Error(t, err)
		require.Nil(t, repo.updated)
	})

	t.Run("invalid bindings are rejected atomically", func(t *testing.T) {
		tests := []struct {
			name    string
			request UpdateAPIKeyRequest
			groups  map[int64]Group
		}{
			{name: "default missing", request: UpdateAPIKeyRequest{GroupIDs: []int64{11, 12}}, groups: multiGroupContractGroups()},
			{name: "nonpositive", request: UpdateAPIKeyRequest{GroupIDs: []int64{0, 2}}, groups: multiGroupContractGroups()},
			{name: "not found", request: UpdateAPIKeyRequest{GroupIDs: []int64{2, 99}}, groups: multiGroupContractGroups()},
			{name: "inactive", request: UpdateAPIKeyRequest{GroupIDs: []int64{2, 11}}, groups: map[int64]Group{
				2:  {ID: 2, Platform: PlatformOpenAI, Status: StatusActive},
				11: {ID: 11, Platform: PlatformAnthropic, Status: StatusDisabled},
			}},
			{name: "inaccessible", request: UpdateAPIKeyRequest{GroupIDs: []int64{2, 11}}, groups: map[int64]Group{
				2:  {ID: 2, Platform: PlatformOpenAI, Status: StatusActive},
				11: {ID: 11, Platform: PlatformAnthropic, Status: StatusActive, IsExclusive: true},
			}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				repo := &multiGroupContractAPIKeyRepo{existing: cloneMultiGroupContractAPIKey(base)}
				service := newMultiGroupContractAPIKeyService(repo, tt.groups)

				updated, err := service.Update(context.Background(), base.ID, base.UserID, tt.request)

				require.Nil(t, updated)
				require.Error(t, err)
				require.Nil(t, repo.updated)
			})
		}
	})
}

type multiGroupContractAPIKeyRepo struct {
	APIKeyRepository
	existing *APIKey
	created  *APIKey
	updated  *APIKey
}

func (r *multiGroupContractAPIKeyRepo) Create(_ context.Context, key *APIKey) error {
	r.created = cloneMultiGroupContractAPIKey(key)
	key.ID = 100
	return nil
}

func (r *multiGroupContractAPIKeyRepo) GetByID(_ context.Context, _ int64) (*APIKey, error) {
	if r.existing == nil {
		return nil, ErrAPIKeyNotFound
	}
	return cloneMultiGroupContractAPIKey(r.existing), nil
}

func (r *multiGroupContractAPIKeyRepo) Update(_ context.Context, key *APIKey) error {
	r.updated = cloneMultiGroupContractAPIKey(key)
	return nil
}

type multiGroupContractUserRepo struct {
	UserRepository
}

func (multiGroupContractUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	return &User{ID: id, Status: StatusActive}, nil
}

type multiGroupContractGroupRepo struct {
	GroupRepository
	groups map[int64]Group
}

func (r multiGroupContractGroupRepo) GetByID(_ context.Context, id int64) (*Group, error) {
	group, ok := r.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	clone := group
	return &clone, nil
}

func newMultiGroupContractAPIKeyService(repo APIKeyRepository, groups map[int64]Group) *APIKeyService {
	return NewAPIKeyService(
		repo,
		multiGroupContractUserRepo{},
		multiGroupContractGroupRepo{groups: groups},
		nil,
		nil,
		nil,
		&config.Config{Default: config.DefaultConfig{APIKeyPrefix: "sk-"}},
	)
}

func multiGroupContractGroups() map[int64]Group {
	return map[int64]Group{
		2:  {ID: 2, Name: "Codex", Platform: PlatformOpenAI, Status: StatusActive},
		11: {ID: 11, Name: "Claude", Platform: PlatformAnthropic, Status: StatusActive},
		12: {ID: 12, Name: "Grok", Platform: PlatformGrok, Status: StatusActive},
	}
}

func cloneMultiGroupContractAPIKey(source *APIKey) *APIKey {
	if source == nil {
		return nil
	}
	return cloneAPIKeyForRequest(source)
}

func cloneAPIKeyGroupPointer(group Group) *Group {
	clone := group
	return &clone
}

func groupsFromMap(groups map[int64]Group, ids []int64) []Group {
	out := make([]Group, 0, len(ids))
	for _, id := range ids {
		out = append(out, groups[id])
	}
	return out
}

func groupIDsFromGroups(groups []Group) []int64 {
	ids := make([]int64, 0, len(groups))
	for i := range groups {
		ids = append(ids, groups[i].ID)
	}
	return ids
}
