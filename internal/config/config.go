// Package config loads private environment settings and legacy credential files.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"
)

type Config struct {
	Language      string `json:"language,omitempty"`
	Timezone      string `json:"timezone,omitempty"`
	PasswordSalt  string `json:"passwordSalt"`
	PasswordHash  string `json:"passwordHash"`
	SessionSecret string `json:"sessionSecret"`
	BotTokenHash  string `json:"botTokenHash"`
}

func Load(path string) (Config, error) {
	var c Config
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fromEnvironment(map[string]string{})
	}
	if err != nil {
		return c, fmt.Errorf("read config: %w", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
		values, err := ParseEnvironment(data)
		if err != nil {
			return c, err
		}
		return fromEnvironment(values)
	}
	if err = json.Unmarshal(data, &c); err != nil {
		return c, errors.New("invalid config JSON")
	}
	return c, validate(c)
}
func validate(c Config) error {
	if c.Language != "" && c.Language != "en" && c.Language != "da" {
		return errors.New("FOODIE_LANGUAGE must be en or da")
	}
	if c.Timezone != "" {
		if _, err := time.LoadLocation(c.Timezone); err != nil {
			return errors.New("invalid FOODIE_TIMEZONE")
		}
	}
	if c.PasswordSalt == "" || len(c.SessionSecret) < 32 {
		return errors.New("invalid password salt or session secret")
	}
	for name, value := range map[string]string{"passwordHash": c.PasswordHash, "botTokenHash": c.BotTokenHash} {
		decoded, err := hex.DecodeString(value)
		expected := 64
		if name == "botTokenHash" {
			expected = 32
		}
		if err != nil || len(decoded) != expected {
			return fmt.Errorf("invalid %s", name)
		}
	}
	return nil
}

// Save replaces a config atomically, retaining its owner and restrictive permissions.
func Save(path string, c Config) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var data []byte
	if strings.HasPrefix(strings.TrimSpace(string(original)), "{") {
		data, err = json.MarshalIndent(c, "", "  ")
		if err != nil {
			return err
		}
	} else {
		values, parseErr := ParseEnvironment(original)
		if parseErr != nil {
			return parseErr
		}
		delete(values, "FOODIE_PASSWORD")
		values["FOODIE_PASSWORD_SALT"] = c.PasswordSalt
		values["FOODIE_PASSWORD_HASH"] = c.PasswordHash
		values["FOODIE_SESSION_SECRET"] = c.SessionSecret
		values["FOODIE_LANGUAGE"] = c.Language
		values["FOODIE_TIMEZONE"] = c.Timezone
		tokenHash := sha256.Sum256([]byte(values["FOODIE_BOT_TOKEN"]))
		if values["FOODIE_BOT_TOKEN"] != "" && hex.EncodeToString(tokenHash[:]) == c.BotTokenHash {
			delete(values, "FOODIE_BOT_TOKEN_HASH")
		} else {
			delete(values, "FOODIE_BOT_TOKEN")
			values["FOODIE_BOT_TOKEN_HASH"] = c.BotTokenHash
		}
		data = encodeEnvironment(values)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = f.Chmod(info.Mode().Perm() & 0640); err != nil {
		return err
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		if err = f.Chown(int(stat.Uid), int(stat.Gid)); err != nil {
			return err
		}
	}
	if _, err = f.Write(append(data, '\n')); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
