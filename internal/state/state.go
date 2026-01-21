package state

import "sync"

const (
	StateNone               = ""
	StateAwaitingScreenshot = "awaiting_screenshot"
	StateAwaitingEmail      = "awaiting_email"
	StateAwaitingPass       = "awaiting_password"
)

type UserSession struct {
	State      string
	Email      string
	Password   string
	PhotoID    string
	AdminMsgID int // ID сообщения в чате админа
}

var Users = sync.Map{}

func GetSession(userID int64) *UserSession {
	val, ok := Users.Load(userID)
	if !ok {
		s := &UserSession{State: StateNone}
		Users.Store(userID, s)
		return s
	}
	return val.(*UserSession)
}
