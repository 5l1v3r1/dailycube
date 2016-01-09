package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/unixpickle/gocube"
)

// PostLoop runs forever, checking if the day changes and
// posting a new scramble whenever it does.
// The postNow channel can be used to trigger premature posts.
func PostLoop(s *State, loc *time.Location, postNow <-chan struct{}) {
	scrambles := scrambleGenerator()
	lastPost := time.Now().In(loc)
	for {
		forcePost := false
		sleepTimeout := time.After(time.Minute / 2)
		select {
		case <-postNow:
			forcePost = true
		case <-sleepTimeout:
		}

		if s.NeedAccessToken() || s.NeedGroupID() {
			continue
		}

		now := time.Now().In(loc)

		if s.DaysRemaining() <= 0 {
			log.Println("Token expired.")
			s.Reset()
			continue
		}

		if forcePost || now.Day() != lastPost.Day() {
			lastPost = now
			postScramble(s, <-scrambles)
		}
	}
}

func postScramble(s *State, scramble []gocube.Move) {
	token, groupID := s.PostInfo()
	if token == "" || groupID == "" {
		log.Println("Missing token or group ID while posting")
		return
	}
	log.Println("Posting scramble:", scramble)
	u := "https://graph.facebook.com/v2.5/" + groupID + "/feed"
	values := url.Values{}
	values.Set("access_token", token)
	values.Set("message", messageForScramble(scramble))
	resp, _ := http.PostForm(u, values)
	if resp != nil {
		resp.Body.Close()
	}
}

func scrambleGenerator() <-chan []gocube.Move {
	rand.Seed(time.Now().UnixNano())
	ch := make(chan []gocube.Move)
	go func() {
		p1Moves := gocube.NewPhase1Moves()
		p1Heuristic := gocube.NewPhase1Heuristic(p1Moves)
		p2Moves := gocube.NewPhase2Moves()
		p2Heuristic := gocube.NewPhase2Heuristic(p2Moves, false)
		tables := gocube.SolverTables{p1Heuristic, p1Moves, p2Heuristic, p2Moves}

		for {
			state := gocube.RandomCubieCube()
			solver := gocube.NewSolverTables(state, 30, tables)
			timeout := time.After(time.Second * 30)
			solution := <-solver.Solutions()

		SolutionLoop:
			for {
				select {
				case sol, ok := <-solver.Solutions():
					if !ok {
						break SolutionLoop
					}
					solution = sol
				case <-timeout:
					break SolutionLoop
				}
			}
			solver.Stop()

			ch <- solution
		}
	}()
	return ch
}

func messageForScramble(scramble []gocube.Move) string {
	solutionStr := fmt.Sprint(scramble)
	solutionStr = solutionStr[1 : len(solutionStr)-1]
	return "Scramble of the day:\n" + solutionStr
}
