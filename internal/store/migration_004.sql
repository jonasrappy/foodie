-- Widen the unit constraint without changing household data or row ordering.
CREATE TABLE items_v4 (
 id TEXT PRIMARY KEY,
 kind TEXT NOT NULL CHECK(kind IN ('shopping','meals')),
 text TEXT NOT NULL,
 checked INTEGER NOT NULL DEFAULT 0,
 version INTEGER NOT NULL DEFAULT 1,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 quantity REAL NOT NULL DEFAULT 1 CHECK(quantity > 0 AND quantity <= 9999),
 unit TEXT NOT NULL DEFAULT 'stk.' CHECK(unit IN ('stk.','liter','milliliter','kilo','gram','pakker','poser','dåser','flasker','bundter','bakker','kasser'))
);
INSERT INTO items_v4(rowid,id,kind,text,checked,version,created_at,updated_at,quantity,unit)
 SELECT rowid,id,kind,text,checked,version,created_at,updated_at,quantity,unit FROM items;
DROP TABLE items;
ALTER TABLE items_v4 RENAME TO items;
PRAGMA user_version=4;
