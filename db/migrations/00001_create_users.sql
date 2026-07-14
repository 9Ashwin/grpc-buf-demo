-- +goose Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    email TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE users;
