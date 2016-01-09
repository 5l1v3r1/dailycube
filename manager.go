package main

import (
	"errors"
	"regexp"
	"sync"
	"time"
)

type Manager struct {
	lock sync.RWMutex

	AccessToken string
	Expiration  time.Time

	GroupID string
}

func (s *Manager) NeedFB() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.AccessToken == ""
}

func (s *Manager) NeedGroup() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.GroupID == ""
}

func (s *Manager) SetGroup(groupURL string) error {
	exp := regexp.MustCompile("https?:\\/\\/(www\\.)?facebook.com\\/groups\\/([0-9]*)\\/?")
	match := exp.FindStringSubmatch(groupURL)
	if len(match) != 3 {
		return errors.New("invalid URL")
	}
	s.lock.Lock()
	s.GroupID = match[2]
	s.lock.Unlock()
	return nil
}

func (s *Manager) SetAccessToken(token string, expireTime int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.AccessToken = token
	s.Expiration = time.Now().Add(time.Second * time.Duration(expireTime))
}

func (s *Manager) Reset() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.AccessToken = ""
	s.GroupID = ""
	s.Expiration = time.Time{}
}

func (s *Manager) DaysRemaining() float64 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return float64(s.Expiration.UnixNano()-time.Now().UnixNano()) / float64(1E9*60*60*24)
}

func (s *Manager) BackgroundRoutine() {
	// TODO: loop here and do magical stuff.
}
