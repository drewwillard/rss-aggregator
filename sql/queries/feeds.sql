-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetFeeds :many
SELECT * FROM feeds;

-- name: GetAllFeedInfo :many
SELECT feeds.name, feeds.url, users.name AS username FROM feeds
JOIN users ON feeds.user_id = users.id;

-- name: GetFeedFromURL :one
SELECT id, name, url, user_id FROM feeds WHERE url = $1;

-- name: MarkFeedFetched :one
UPDATE feeds
SET updated_at = $2, last_fetched_at = $3
WHERE feeds.id = $1
RETURNING *;

-- name: GetNextFeedToFetch :one
SELECT id, name, url, user_id, last_fetched_at
FROM feeds
ORDER BY last_fetched_at NULLS FIRST, last_fetched_at ASC
LIMIT 1;
