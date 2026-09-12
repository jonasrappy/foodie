-- English unit IDs replace localized identifiers without touching item text.
CREATE TABLE items_v5 (
 id TEXT PRIMARY KEY,
 kind TEXT NOT NULL CHECK(kind IN ('shopping','meals')),
 text TEXT NOT NULL,
 checked INTEGER NOT NULL DEFAULT 0,
 version INTEGER NOT NULL DEFAULT 1,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 quantity REAL NOT NULL DEFAULT 1 CHECK(quantity > 0 AND quantity <= 9999),
 unit TEXT NOT NULL DEFAULT 'piece' CHECK(unit IN ('piece','liter','milliliter','kilogram','gram','pack','bag','can','bottle','bunch','tray','crate'))
);
