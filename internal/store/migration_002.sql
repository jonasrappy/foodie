ALTER TABLE items ADD COLUMN quantity REAL NOT NULL DEFAULT 1 CHECK(quantity > 0 AND quantity <= 9999);
ALTER TABLE items ADD COLUMN unit TEXT NOT NULL DEFAULT 'stk.' CHECK(unit IN ('stk.','liter','milliliter','kilo','gram','pakker','poser','dåser','flasker','bundter','bakker'));
PRAGMA user_version=2;
