CREATE TABLE IF NOT EXISTS voice_transcripts (
 id TEXT PRIMARY KEY,
 fingerprint TEXT NOT NULL,
 transcript TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS voice_intents (
 id TEXT PRIMARY KEY,
 command TEXT NOT NULL,
 signature TEXT NOT NULL,
 response TEXT NOT NULL,
 done INTEGER NOT NULL DEFAULT 0,
 created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS voice_replies (id TEXT PRIMARY KEY, response TEXT NOT NULL);
PRAGMA user_version=3;
