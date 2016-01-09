package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/unixpickle/gocube"
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
	lastPost := time.Now()
	for {
		time.Sleep(time.Minute / 2)
		if s.NeedFB() || s.NeedGroup() {
			continue
		}
		now := time.Now()

		if now.After(s.Expiration) {
			log.Println("Token expired!")
			s.Reset()
			continue
		}

		if now.Day() != lastPost.Day() {
			lastPost = now
			s.postScramble()
		}
	}
}

func (s *Manager) postScramble() {
	rand.Seed(time.Now().UnixNano())

	log.Println("Posting scramble...")

	s.lock.RLock()
	defer s.lock.RUnlock()
	if s.NeedFB() || s.NeedGroup() {
		return
	}

	p1Moves := gocube.NewPhase1Moves()
	p1Heuristic := gocube.NewPhase1Heuristic(p1Moves)
	p2Moves := gocube.NewPhase2Moves()
	p2Heuristic := gocube.NewPhase2Heuristic(p2Moves, false)
	tables := gocube.SolverTables{p1Heuristic, p1Moves, p2Heuristic, p2Moves}

	state := gocube.RandomCubieCube()
	solver := gocube.NewSolverTables(state, 30, tables)
	timeout := time.After(time.Second * 30)
	solution := <-solver.Solutions()

BetterLoop:
	for {
		select {
		case sol, ok := <-solver.Solutions():
			if !ok {
				break BetterLoop
			}
			solution = sol
		case <-timeout:
			break BetterLoop
		}
	}
	solver.Stop()

	solutionStr := fmt.Sprint(solution)
	solutionStr = solutionStr[1 : len(solutionStr)-1]

	log.Println("Scramble is:", solutionStr)

	u := "https://graph.facebook.com/v2.5/" + s.GroupID + "/feed"
	values := url.Values{}
	values.Set("access_token", s.AccessToken)
	values.Set("message", "Scramble of the day: "+solutionStr)
	resp, _ := http.PostForm(u, values)
	if resp != nil {
		resp.Body.Close()
	}
}
