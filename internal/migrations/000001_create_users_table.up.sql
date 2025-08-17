create table if not exists users
(
    id       uuid PRIMARY KEY,
    login    varchar(255) NOT NULL UNIQUE,
    password varchar(512) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);