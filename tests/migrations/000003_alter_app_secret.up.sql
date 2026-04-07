ALTER TABLE apps 
ALTER COLUMN secret TYPE BYTEA 
USING secret::bytea;