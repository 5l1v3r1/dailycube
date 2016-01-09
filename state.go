package main

import (
	"sync"
	"time"
)

// State manages the information that the server uses to post
// things to Facebook.
type State struct {
	lock sync.RWMutex

	accessToken string
	expiration  time.Time

	groupID string
}

// NeedAccessToken returns true if no access token is configured.
func (s *State) NeedAccessToken() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.accessToken == ""
}

// NeedGroupID returns true if no group ID is configured.
func (s *State) NeedGroupID() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.groupID == ""
}

// SetGroupID updates the state's group ID
func (s *State) SetGroupID(groupID string) error {
	s.lock.Lock()
	s.groupID = groupID
	s.lock.Unlock()
	return nil
}

// SetAccessToken updates the state's access token.
func (s *State) SetAccessToken(token string, expireTime int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.accessToken = token
	s.expiration = time.Now().Add(time.Second * time.Duration(expireTime))
}

// Reset deletes the current access token and group ID.
func (s *State) Reset() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.accessToken = ""
	s.groupID = ""
	s.expiration = time.Time{}
}

// DaysRemaining returns the number of days until the expiration date.
// This may be negative.
func (s *State) DaysRemaining() float64 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return float64(s.expiration.UnixNano()-time.Now().UnixNano()) / float64(1E9*60*60*24)
}

// PostInfo returns the access token and group ID atomically.
func (s *State) PostInfo() (token, groupID string) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.accessToken, s.groupID
}
