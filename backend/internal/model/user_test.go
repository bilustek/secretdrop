package model_test

import (
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/model"
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
