-- name: ListChannelsForCheck :many
SELECT channel_id FROM check_notification_channels
WHERE check_id = ?
ORDER BY channel_id ASC;

-- name: ListChannelRowsForCheck :many
SELECT nc.*
FROM check_notification_channels cnc
JOIN notification_channels nc ON nc.id = cnc.channel_id
WHERE cnc.check_id = ?
ORDER BY nc.name ASC;

-- name: ListChecksForChannel :many
SELECT check_id FROM check_notification_channels
WHERE channel_id = ?
ORDER BY check_id ASC;

-- name: LinkCheckToChannel :exec
INSERT OR IGNORE INTO check_notification_channels (check_id, channel_id)
VALUES (?, ?);

-- name: UnlinkCheckFromChannel :exec
DELETE FROM check_notification_channels
WHERE check_id = ? AND channel_id = ?;

-- name: ClearChannelsForCheck :exec
DELETE FROM check_notification_channels WHERE check_id = ?;
