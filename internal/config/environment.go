package config

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"golang.org/x/crypto/scrypt"
)

var environmentKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// ParseEnvironment never executes shell commands or expands variables. Errors
// identify a line, never its value: a malformed secret must not enter logs.
func ParseEnvironment(data []byte) (map[string]string, error) {
	values := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 4096), 65536)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, value, ok := strings.Cut(text, "=")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !ok || !environmentKey.MatchString(key) {
			return nil, fmt.Errorf("invalid environment entry on line %d", line)
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("duplicate environment key on line %d", line)
		}
		if strings.HasPrefix(value, `"`) {
			decoded, err := strconv.Unquote(value)
			if err != nil {
				return nil, fmt.Errorf("invalid quoted value on line %d", line)
			}
			value = decoded
		} else if strings.HasPrefix(value, "'") {
			if len(value) < 2 || !strings.HasSuffix(value, "'") {
				return nil, fmt.Errorf("invalid quoted value on line %d", line)
			}
			value = value[1 : len(value)-1]
		}
		if strings.ContainsAny(value, "\x00\r\n") {
			return nil, fmt.Errorf("multiline or binary value on line %d", line)
		}
		values[key] = value
	}
	if scanner.Err() != nil {
		return nil, errors.New("environment file contains an oversized or unreadable line")
	}
	return values, nil
}

func ReadEnvironment(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
		return map[string]string{}, nil
	}
	return ParseEnvironment(data)
}
func Value(values map[string]string, key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	if value, ok := values[key]; ok {
		return value
	}
	return fallback
}
func encodeEnvironment(values map[string]string) []byte {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out strings.Builder
	out.WriteString("# Foodie local configuration. Keep this file private and out of Git.\n")
	for _, key := range keys {
		fmt.Fprintf(&out, "%s=%s\n", key, strconv.Quote(values[key]))
	}
	return []byte(out.String())
}
func DerivePassword(password, salt string) (string, error) {
	key, err := scrypt.Key([]byte(password), []byte(salt), 16384, 8, 1, 64)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}
func fromEnvironment(values map[string]string) (Config, error) {
	c := Config{Language: Value(values, "FOODIE_LANGUAGE", "en"), Timezone: Value(values, "FOODIE_TIMEZONE", "UTC"), PasswordSalt: Value(values, "FOODIE_PASSWORD_SALT", ""), PasswordHash: Value(values, "FOODIE_PASSWORD_HASH", ""), SessionSecret: Value(values, "FOODIE_SESSION_SECRET", ""), BotTokenHash: Value(values, "FOODIE_BOT_TOKEN_HASH", "")}
	password, bot := Value(values, "FOODIE_PASSWORD", ""), Value(values, "FOODIE_BOT_TOKEN", "")
	if password != "" {
		if c.PasswordHash != "" {
			return c, errors.New("configure FOODIE_PASSWORD or FOODIE_PASSWORD_HASH, not both")
		}
		if length := len(utf16.Encode([]rune(password))); length < 8 || length > 200 {
			return c, errors.New("FOODIE_PASSWORD must contain 8–200 characters")
		}
		var err error
		c.PasswordHash, err = DerivePassword(password, c.PasswordSalt)
		if err != nil {
			return c, err
		}
	}
	if bot != "" {
		if c.BotTokenHash != "" {
			return c, errors.New("configure FOODIE_BOT_TOKEN or FOODIE_BOT_TOKEN_HASH, not both")
		}
		if len(bot) < 32 || len(bot) > 1024 {
			return c, errors.New("FOODIE_BOT_TOKEN must contain 32–1024 bytes")
		}
		hash := sha256.Sum256([]byte(bot))
		c.BotTokenHash = hex.EncodeToString(hash[:])
	}
	if err := validate(c); err != nil {
		return c, fmt.Errorf("invalid credentials; run foodie init or configure your environment: %w", err)
	}
	return c, nil
}

// Generate writes random, independent credentials exactly once, with mode 0600.
func Generate(path string) error {
	random := func() (string, error) {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(b), nil
	}
	values := map[string]string{"FOODIE_LANGUAGE": "en", "FOODIE_TIMEZONE": "UTC", "FOODIE_LISTEN": "127.0.0.1:8082", "FOODIE_DATA_DIR": "./data", "FOODIE_PUBLIC_DIR": "./public", "FOODIE_SITE_URL": "https://foodie.example.com", "ANDROID_APPLICATION_ID": "app.foodie.mobile", "ANDROID_SIGN_DIR": "./.private/android-signing", "ANDROID_KEY_ALIAS": "foodie", "ANDROID_VERSION_NAME": "1.3.1", "ANDROID_VERSION_CODE": "7"}
	for _, key := range []string{"FOODIE_PASSWORD", "FOODIE_PASSWORD_SALT", "FOODIE_SESSION_SECRET", "FOODIE_BOT_TOKEN"} {
		value, err := random()
		if err != nil {
			return err
		}
		values[key] = value
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create environment file (existing files are never overwritten): %w", err)
	}
	complete := false
	defer func() {
		f.Close()
		if !complete {
			os.Remove(path)
		}
	}()
	if _, err = f.Write(encodeEnvironment(values)); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	complete = true
	return nil
}
