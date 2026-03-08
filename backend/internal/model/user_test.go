package model_test

import (
	"testing"

	"github.com/bilustek/secretdrop/internal/model"
)

func TestUserSecretsLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tier string
		want int
	}{
		{
			name: "free tier returns 5",
			tier: model.TierFree,
			want: model.FreeTierLimit,
		},
		{
			name: "pro tier returns 100",
			tier: model.TierPro,
			want: model.ProTierLimit,
		},
		{
			name: "team tier returns 1000",
			tier: model.TierTeam,
			want: model.TeamTierLimit,
		},
		{
			name: "unknown tier defaults to free limit",
			tier: "unknown",
			want: model.FreeTierLimit,
		},
		{
			name: "empty tier defaults to free limit",
			tier: "",
			want: model.FreeTierLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u := &model.User{Tier: tt.tier}

			if got := u.SecretsLimit(); got != tt.want {
				t.Errorf("SecretsLimit() = %d; want %d", got, tt.want)
			}
		})
	}
}

func intPtr(v int) *int { return &v }

func TestUserSecretsLimit_WithOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		tier             string
		tierSecretsLimit int
		secretsLimitOvrd *int
		want             int
	}{
		{
			name:             "override on free tier with no tier limit",
			tier:             model.TierFree,
			tierSecretsLimit: 0,
			secretsLimitOvrd: intPtr(500),
			want:             500,
		},
		{
			name:             "override on pro tier with tier limit loaded",
			tier:             model.TierPro,
			tierSecretsLimit: 100,
			secretsLimitOvrd: intPtr(1000),
			want:             1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u := &model.User{
				Tier:                 tt.tier,
				TierSecretsLimit:     tt.tierSecretsLimit,
				SecretsLimitOverride: tt.secretsLimitOvrd,
			}

			if got := u.SecretsLimit(); got != tt.want {
				t.Errorf("SecretsLimit() = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestUserSecretsLimit_WithTierLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		tier             string
		tierSecretsLimit int
		want             int
	}{
		{
			name:             "tier limit on free tier",
			tier:             model.TierFree,
			tierSecretsLimit: 50,
			want:             50,
		},
		{
			name:             "tier limit on pro tier",
			tier:             model.TierPro,
			tierSecretsLimit: 200,
			want:             200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u := &model.User{
				Tier:             tt.tier,
				TierSecretsLimit: tt.tierSecretsLimit,
			}

			if got := u.SecretsLimit(); got != tt.want {
				t.Errorf("SecretsLimit() = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestUserSecretsLimit_PriorityChain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		tier             string
		tierSecretsLimit int
		secretsLimitOvrd *int
		want             int
	}{
		{
			name:             "override wins over tier limit and fallback",
			tier:             model.TierFree,
			tierSecretsLimit: 50,
			secretsLimitOvrd: intPtr(500),
			want:             500,
		},
		{
			name:             "tier limit wins over fallback",
			tier:             model.TierFree,
			tierSecretsLimit: 50,
			secretsLimitOvrd: nil,
			want:             50,
		},
		{
			name:             "pro fallback when no override or tier limit",
			tier:             model.TierPro,
			tierSecretsLimit: 0,
			secretsLimitOvrd: nil,
			want:             model.ProTierLimit,
		},
		{
			name:             "free fallback when no override or tier limit",
			tier:             model.TierFree,
			tierSecretsLimit: 0,
			secretsLimitOvrd: nil,
			want:             model.FreeTierLimit,
		},
		{
			name:             "team fallback when no override or tier limit",
			tier:             model.TierTeam,
			tierSecretsLimit: 0,
			secretsLimitOvrd: nil,
			want:             model.TeamTierLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u := &model.User{
				Tier:                 tt.tier,
				TierSecretsLimit:     tt.tierSecretsLimit,
				SecretsLimitOverride: tt.secretsLimitOvrd,
			}

			if got := u.SecretsLimit(); got != tt.want {
				t.Errorf("SecretsLimit() = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestUserMaxTextLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tier string
		want int
	}{
		{
			name: "free tier returns 4096",
			tier: model.TierFree,
			want: model.FreeMaxTextLength,
		},
		{
			name: "pro tier returns 65536",
			tier: model.TierPro,
			want: model.ProMaxTextLength,
		},
		{
			name: "team tier returns 262144",
			tier: model.TierTeam,
			want: model.TeamMaxTextLength,
		},
		{
			name: "unknown tier defaults to free",
			tier: "unknown",
			want: model.FreeMaxTextLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u := &model.User{Tier: tt.tier}

			if got := u.MaxTextLength(); got != tt.want {
				t.Errorf("MaxTextLength() = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestUserRecipientsLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		tier                string
		tierRecipientsLimit int
		want                int
	}{
		{
			name:                "tier recipients limit overrides fallback",
			tier:                model.TierFree,
			tierRecipientsLimit: 20,
			want:                20,
		},
		{
			name:                "pro fallback when no tier recipients limit",
			tier:                model.TierPro,
			tierRecipientsLimit: 0,
			want:                model.ProMaxRecipients,
		},
		{
			name:                "team fallback when no tier recipients limit",
			tier:                model.TierTeam,
			tierRecipientsLimit: 0,
			want:                model.TeamMaxRecipients,
		},
		{
			name:                "free fallback when no tier recipients limit",
			tier:                model.TierFree,
			tierRecipientsLimit: 0,
			want:                model.FreeMaxRecipients,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u := &model.User{
				Tier:                tt.tier,
				TierRecipientsLimit: tt.tierRecipientsLimit,
			}

			if got := u.RecipientsLimit(); got != tt.want {
				t.Errorf("RecipientsLimit() = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestUserCanCreateSecret_WithOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		secretsLimitOvrd *int
		secretsUsed      int
		want             bool
	}{
		{
			name:             "under override limit",
			secretsLimitOvrd: intPtr(1000),
			secretsUsed:      500,
			want:             true,
		},
		{
			name:             "at override limit",
			secretsLimitOvrd: intPtr(1000),
			secretsUsed:      1000,
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u := &model.User{
				Tier:                 model.TierFree,
				SecretsLimitOverride: tt.secretsLimitOvrd,
				SecretsUsed:          tt.secretsUsed,
			}

			if got := u.CanCreateSecret(); got != tt.want {
				t.Errorf("CanCreateSecret() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestUserCanCreateSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tier        string
		secretsUsed int
		want        bool
	}{
		{
			name:        "free tier with no secrets used",
			tier:        model.TierFree,
			secretsUsed: 0,
			want:        true,
		},
		{
			name:        "free tier at limit",
			tier:        model.TierFree,
			secretsUsed: model.FreeTierLimit,
			want:        false,
		},
		{
			name:        "pro tier under limit",
			tier:        model.TierPro,
			secretsUsed: 50,
			want:        true,
		},
		{
			name:        "pro tier at limit",
			tier:        model.TierPro,
			secretsUsed: model.ProTierLimit,
			want:        false,
		},
		{
			name:        "team tier under limit",
			tier:        model.TierTeam,
			secretsUsed: 500,
			want:        true,
		},
		{
			name:        "team tier at limit",
			tier:        model.TierTeam,
			secretsUsed: model.TeamTierLimit,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u := &model.User{
				Tier:        tt.tier,
				SecretsUsed: tt.secretsUsed,
			}

			if got := u.CanCreateSecret(); got != tt.want {
				t.Errorf("CanCreateSecret() = %v; want %v", got, tt.want)
			}
		})
	}
}
