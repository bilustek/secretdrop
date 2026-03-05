package user_test

import (
	"testing"

	"github.com/bilustek/secretdrop/internal/user"
)

func TestDefaultListQuery(t *testing.T) {
	t.Parallel()

	q := user.DefaultListQuery()

	if q.Sort != "created_at" {
		t.Errorf("Sort = %q; want %q", q.Sort, "created_at")
	}

	if q.Order != "desc" {
		t.Errorf("Order = %q; want %q", q.Order, "desc")
	}

	if q.Page != 1 {
		t.Errorf("Page = %d; want 1", q.Page)
	}

	if q.PerPage != user.DefaultPerPage {
		t.Errorf("PerPage = %d; want %d", q.PerPage, user.DefaultPerPage)
	}

	if q.Search != "" {
		t.Errorf("Search = %q; want empty", q.Search)
	}

	if q.Tier != "" {
		t.Errorf("Tier = %q; want empty", q.Tier)
	}

	if q.Status != "" {
		t.Errorf("Status = %q; want empty", q.Status)
	}
}

func TestApplyOptions_NoOptions(t *testing.T) {
	t.Parallel()

	q := user.ApplyOptions()

	if q.Sort != "created_at" {
		t.Errorf("Sort = %q; want %q", q.Sort, "created_at")
	}

	if q.Page != 1 {
		t.Errorf("Page = %d; want 1", q.Page)
	}
}

func TestWithSearch(t *testing.T) {
	t.Parallel()

	q := user.ApplyOptions(user.WithSearch("alice"))

	if q.Search != "alice" {
		t.Errorf("Search = %q; want %q", q.Search, "alice")
	}
}

func TestWithTier(t *testing.T) {
	t.Parallel()

	q := user.ApplyOptions(user.WithTier("pro"))

	if q.Tier != "pro" {
		t.Errorf("Tier = %q; want %q", q.Tier, "pro")
	}
}

func TestWithProvider(t *testing.T) {
	t.Parallel()

	q := user.ApplyOptions(user.WithProvider("github"))

	if q.Provider != "github" {
		t.Errorf("Provider = %q; want %q", q.Provider, "github")
	}
}

func TestWithStatus(t *testing.T) {
	t.Parallel()

	q := user.ApplyOptions(user.WithStatus("active"))

	if q.Status != "active" {
		t.Errorf("Status = %q; want %q", q.Status, "active")
	}
}

func TestWithSort(t *testing.T) {
	t.Parallel()

	q := user.ApplyOptions(user.WithSort("email", "asc"))

	if q.Sort != "email" {
		t.Errorf("Sort = %q; want %q", q.Sort, "email")
	}

	if q.Order != "asc" {
		t.Errorf("Order = %q; want %q", q.Order, "asc")
	}
}

func TestWithPage(t *testing.T) {
	t.Parallel()

	q := user.ApplyOptions(user.WithPage(3, 50))

	if q.Page != 3 {
		t.Errorf("Page = %d; want 3", q.Page)
	}

	if q.PerPage != 50 {
		t.Errorf("PerPage = %d; want 50", q.PerPage)
	}
}

func TestApplyOptions_Multiple(t *testing.T) {
	t.Parallel()

	q := user.ApplyOptions(
		user.WithSearch("test"),
		user.WithTier("free"),
		user.WithSort("name", "asc"),
		user.WithPage(2, 10),
	)

	if q.Search != "test" {
		t.Errorf("Search = %q; want %q", q.Search, "test")
	}

	if q.Tier != "free" {
		t.Errorf("Tier = %q; want %q", q.Tier, "free")
	}

	if q.Sort != "name" {
		t.Errorf("Sort = %q; want %q", q.Sort, "name")
	}

	if q.Order != "asc" {
		t.Errorf("Order = %q; want %q", q.Order, "asc")
	}

	if q.Page != 2 {
		t.Errorf("Page = %d; want 2", q.Page)
	}

	if q.PerPage != 10 {
		t.Errorf("PerPage = %d; want 10", q.PerPage)
	}
}
