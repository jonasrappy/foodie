package config_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonasrappy/foodie/internal/auth"
	"github.com/jonasrappy/foodie/internal/config"
)

func TestGenerateUniquePrivateCredentialsAndRotatePassword(t *testing.T) {
	paths := []string{filepath.Join(t.TempDir(), "first.env"), filepath.Join(t.TempDir(), "second.env")}
	var firstValues map[string]string
	for i, path := range paths {
		if err := config.Generate(path); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0600 {
			t.Fatal("credentials not private")
		}
		before, _ := os.ReadFile(path)
		if err := config.Generate(path); err == nil {
			t.Fatal("overwrote existing credentials")
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("changed an existing file")
		}
		values, err := config.ReadEnvironment(path)
		if err != nil {
			t.Fatal(err)
		}
		c, err := config.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		a := auth.New(c)
		if !a.CheckPassword(values["FOODIE_PASSWORD"]) || a.Authenticate("Bearer "+values["FOODIE_BOT_TOKEN"]) != auth.Bot {
			t.Fatal("generated credentials do not authenticate")
		}
		if i == 0 {
			firstValues = values
		} else {
			for _, key := range []string{"FOODIE_PASSWORD", "FOODIE_PASSWORD_SALT", "FOODIE_SESSION_SECRET", "FOODIE_BOT_TOKEN"} {
				if values[key] == firstValues[key] {
					t.Fatal("credentials reused")
				}
			}
		}
		device, err := a.IssueToken()
		if err != nil {
			t.Fatal(err)
		}
		c.PasswordSalt = "a-new-independent-test-salt"
		c.PasswordHash, err = auth.HashPassword("a replacement test password", c.PasswordSalt)
		if err != nil {
			t.Fatal(err)
		}
		if err = config.Save(path, c); err != nil {
			t.Fatal(err)
		}
		reloaded, err := config.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		rotated := auth.New(reloaded)
		if rotated.Authenticate("Bearer "+device) != auth.Unauthenticated || rotated.Authenticate("Bearer "+values["FOODIE_BOT_TOKEN"]) != auth.Bot {
			t.Fatal("rotation must revoke devices but preserve the bot")
		}
		updated, _ := config.ReadEnvironment(path)
		if updated["FOODIE_SITE_URL"] != values["FOODIE_SITE_URL"] || updated["FOODIE_BOT_TOKEN"] != values["FOODIE_BOT_TOKEN"] || updated["FOODIE_PASSWORD"] != "" {
			t.Fatal("rotation lost unrelated settings or retained the old password")
		}
	}
}
func TestLiteralEnvironmentAndRedactedFailures(t *testing.T) {
	values, err := config.ParseEnvironment([]byte("# comment\nFOODIE_PASSWORD='$(must-never-run) # literal'\nFOODIE_SITE_URL=\"https://foodie.example.com\"\n"))
	if err != nil || values["FOODIE_PASSWORD"] != "$(must-never-run) # literal" {
		t.Fatal("parser evaluated or changed a literal")
	}
	for _, input := range []string{"SECRET=\"private-value", "SECRET=one\nSECRET=private-value", "private-value", "SECRET=\"private-value\\nsecond\""} {
		_, err := config.ParseEnvironment([]byte(input))
		if err == nil || strings.Contains(err.Error(), "private-value") {
			t.Fatal("invalid input accepted or leaked")
		}
	}
}
func TestEnvironmentOverridesAndMissingSecretsFailClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.env")
	if _, err := config.Load(path); err == nil {
		t.Fatal("missing credentials accepted")
	}
	if err := config.Generate(path); err != nil {
		t.Fatal(err)
	}
	values, _ := config.ReadEnvironment(path)
	t.Setenv("FOODIE_PASSWORD", "another isolated test password")
	c, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !auth.New(c).CheckPassword("another isolated test password") || auth.New(c).CheckPassword(values["FOODIE_PASSWORD"]) {
		t.Fatal("process environment did not override file")
	}
	t.Setenv("FOODIE_PASSWORD", strings.Repeat("x", 201))
	if _, err := config.Load(path); err == nil {
		t.Fatal("oversized password accepted")
	}
}
