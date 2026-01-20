package userctx

import "sync"

// Singleton for current user (константа).
var (
	userOnce sync.Once
	userID   uint = 1
)

// CurrentUserID returns fixed user id for this lab.
func CurrentUserID() uint {
	userOnce.Do(func() {
		// could load from config/env if needed
	})
	return userID
}

// CurrentModeratorID returns fixed moderator id for moderation actions.
func CurrentModeratorID() uint {
	return 3
}
