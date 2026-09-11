package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jonasrappy/foodie/internal/i18n"
	"github.com/jonasrappy/foodie/internal/voice"
)

type VoiceResponse struct {
	Kind           string `json:"kind"`
	Speech         string `json:"speech"`
	ConfirmationID string `json:"confirmation_id,omitempty"`
}

// VoiceTranscript binds an idempotency key to the exact audio and conversation.
// Audio itself is never stored. A lost HTTP response can reuse the transcript.
func (s *Store) VoiceTranscript(ctx context.Context, id, fingerprint string, save *string) (string, bool, error) {
	if !requestID.MatchString(id) {
		return "", false, Invalid("Ugyldig forespørgsel.")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	var old, text string
	err = tx.QueryRowContext(ctx, "SELECT fingerprint,transcript FROM voice_transcripts WHERE id=?", id).Scan(&old, &text)
	if err == nil {
		if old != fingerprint {
			return "", false, Conflict("Forespørgslen er allerede brugt.")
		}
		return text, true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	if save == nil {
		return "", false, nil
	}
	if len(*save) > 2000 {
		return "", false, Invalid("Kommandoen er for lang.")
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO voice_transcripts VALUES(?,?,?)", id, fingerprint, *save)
	if err != nil {
		return "", false, err
	}
	return *save, true, tx.Commit()
}
func spokenNumber(n float64) string {
	return strings.ReplaceAll(strconv.FormatFloat(math.Round(n*100)/100, 'f', -1, 64), ".", ",")
}
func voiceMatches(state State, c voice.Command) ([]Item, string) {
	rows := []Item{}
	for _, item := range state.Shopping {
		if strings.EqualFold(strings.Join(strings.Fields(item.Text), " "), strings.Join(strings.Fields(c.Text), " ")) {
			rows = append(rows, item)
		}
	}
	encoded, _ := json.Marshal(rows)
	hash := sha256.Sum256(encoded)
	return rows, hex.EncodeToString(hash[:])
}
func duplicateQuestion(c voice.Command, rows []Item) (string, *Item) {
	return duplicateQuestionLocalized(c, rows, "da")
}
func duplicateQuestionLocalized(c voice.Command, rows []Item, language string) (string, *Item) {
	number := func(n float64) string { return i18n.Number(language, n) }
	unit := func(u string, n float64) string { return i18n.Unit(language, u, n) }
	var target *Item
	total := 0.0
	for i := range rows {
		if !rows[i].Checked && rows[i].Unit == c.Unit {
			total += rows[i].Quantity
			if target == nil {
				copy := rows[i]
				target = &copy
			}
		}
	}
	if target != nil {
		extra := fmt.Sprintf(i18n.Text(language, "%s ekstra"), number(c.Quantity))
		if c.Quantity == 1 {
			extra = i18n.Text(language, "en ekstra")
		}
		return fmt.Sprintf(i18n.Text(language, "Der er allerede %s %s %s på indkøbslisten. Skal jeg tilføje %s, så det bliver %s %s?"), number(total), unit(c.Unit, total), c.Text, extra, number(total+c.Quantity), unit(c.Unit, total+c.Quantity)), target
	}
	if len(rows) > 0 {
		item := rows[0]
		if item.Checked {
			return fmt.Sprintf(i18n.Text(language, "%s er allerede krydset af på indkøbslisten. Skal jeg tilføje %s %s som en ny vare?"), c.Text, number(c.Quantity), unit(c.Unit, c.Quantity)), nil
		}
		return fmt.Sprintf(i18n.Text(language, "Der er allerede %s %s %s på indkøbslisten. Skal jeg tilføje %s %s som en ny linje?"), number(item.Quantity), unit(item.Unit, item.Quantity), c.Text, number(c.Quantity), unit(c.Unit, c.Quantity)), nil
	}
	return fmt.Sprintf(i18n.Text(language, "Listen er ændret. Skal jeg tilføje %s %s %s til indkøbslisten?"), number(c.Quantity), unit(c.Unit, c.Quantity), c.Text), nil
}

// ApplyVoice plans or commits in one SQLite transaction. A confirmation is tied
// to the matching row versions; concurrent edits always produce a fresh question.
func (s *Store) ApplyVoice(ctx context.Context, id string, replyID string, newCommand *voice.Command, answer *bool) (VoiceResponse, State, error) {
	return s.ApplyVoiceLocalized(ctx, id, replyID, newCommand, answer, "da")
}
func (s *Store) ApplyVoiceLocalized(ctx context.Context, id string, replyID string, newCommand *voice.Command, answer *bool, language string) (VoiceResponse, State, error) {
	var response VoiceResponse
	if !requestID.MatchString(id) || !requestID.MatchString(replyID) {
		return response, State{}, Invalid("Ugyldig forespørgsel.")
	}
	state, err := s.write(ctx, func(tx *sql.Tx) (bool, error) {
		var replay string
		replayErr := tx.QueryRowContext(ctx, "SELECT response FROM voice_replies WHERE id=?", replyID).Scan(&replay)
		if replayErr == nil {
			return false, json.Unmarshal([]byte(replay), &response)
		}
		if !errors.Is(replayErr, sql.ErrNoRows) {
			return false, replayErr
		}
		var encoded, signature, cached string
		var done bool
		var created int64
		err := tx.QueryRowContext(ctx, "SELECT command,signature,response,done,created_at FROM voice_intents WHERE id=?", id).Scan(&encoded, &signature, &cached, &done, &created)
		fresh := errors.Is(err, sql.ErrNoRows)
		if err != nil && !fresh {
			return false, err
		}
		var command voice.Command
		if fresh {
			if newCommand == nil {
				return false, Invalid("Spørg mig igen om varen.")
			}
			command = *newCommand
			raw, _ := json.Marshal(command)
			encoded = string(raw)
			created = time.Now().Unix()
		} else {
			if err = json.Unmarshal([]byte(encoded), &command); err != nil {
				return false, err
			}
			if done {
				return false, json.Unmarshal([]byte(cached), &response)
			}
			if time.Now().Unix()-created > 120 {
				return false, Invalid("Bekræftelsen er udløbet. Spørg mig igen om varen.")
			}
		}
		if _, err = CleanText(command.Text); err != nil {
			return false, err
		}
		if err = ValidateAmount(command.Quantity, command.Unit); err != nil {
			return false, err
		}
		nowState, err := snapshot(ctx, tx)
		if err != nil {
			return false, err
		}
		matches, currentSignature := voiceMatches(nowState, command)
		question, target := duplicateQuestionLocalized(command, matches, language)
		changed := false
		if answer != nil && !*answer {
			response = VoiceResponse{Kind: "cancelled", Speech: i18n.Text(language, "Okay. Jeg ændrer ikke indkøbslisten.")}
			done = true
		} else if (fresh && len(matches) == 0) || (answer != nil && *answer && signature == currentSignature) {
			if target != nil {
				next := math.Round((target.Quantity+command.Quantity)*100) / 100
				if err = ValidateAmount(next, command.Unit); err != nil {
					return false, err
				}
				_, err = tx.ExecContext(ctx, "UPDATE items SET quantity=?,version=version+1,updated_at=? WHERE id=?", next, now(), target.ID)
			} else {
				if len(nowState.Shopping)+len(nowState.Meals) >= 1000 {
					return false, Invalid("Listen er fuld. Fjern nogle linjer først.")
				}
				random := make([]byte, 16)
				if _, err = rand.Read(random); err != nil {
					return false, err
				}
				stamp := now()
				_, err = tx.ExecContext(ctx, "INSERT INTO items(id,kind,text,created_at,updated_at,quantity,unit) VALUES(?,'shopping',?,?,?,?,?)", hex.EncodeToString(random), command.Text, stamp, stamp, command.Quantity, command.Unit)
			}
			if err != nil {
				return false, err
			}
			changed = true
			done = true
			response = VoiceResponse{Kind: "added", Speech: command.ReplyLocalized(language)}
		} else {
			if !fresh && signature != currentSignature && len(matches) > 0 {
				question = i18n.Text(language, "Listen er ændret. ") + question
			}
			response = VoiceResponse{Kind: "confirm", Speech: question, ConfirmationID: id}
		}
		raw, _ := json.Marshal(response)
		_, err = tx.ExecContext(ctx, `INSERT INTO voice_intents(id,command,signature,response,done,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET signature=excluded.signature,response=excluded.response,done=excluded.done`, id, encoded, currentSignature, string(raw), done, created)
		if err != nil {
			return false, err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO voice_replies VALUES(?,?)", replyID, string(raw))
		return changed, err
	})
	return response, state, err
}
