package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/howeyc/gopass"
)

type Config struct {
	AdminHash   string `json:"admin_hash"`
	ClientID    string `json:"client_id"`
	Secret      string `json:"secret"`
	CallbackURI string `json:"callback_uri"`
	Timezone    string `json:"timezone"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: genconfig <output.json>")
		os.Exit(1)
	}

	var c Config

	fmt.Print("Admin password: ")
	pass := gopass.GetPasswd()
	hash := sha256.Sum256([]byte(pass))
	c.AdminHash = strings.ToLower(hex.EncodeToString(hash[:]))

	c.ClientID = promptInput("FB App ID: ")
	c.Secret = promptInput("FB Secret: ")
	c.CallbackURI = promptInput("Landing URL (e.g. http://foo.com): ") + "/fblogin_done"

	for {
		tz := promptInput("Time zone (e.g. UTC, America/New_York): ")
		if _, err := time.LoadLocation(tz); err != nil {
			fmt.Println("Invalid time zone")
		} else {
			c.Timezone = tz
			break
		}
	}

	data, _ := json.Marshal(&c)
	if err := ioutil.WriteFile(os.Args[1], data, 0700); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to write output:", err)
		os.Exit(1)
	}
}

func promptInput(prompt string) string {
	fmt.Print(prompt)
	return readLine()
}

func readLine() string {
	res := make([]byte, 0, 100)
	var bytes [1]byte
	for {
		if _, err := os.Stdin.Read(bytes[:]); err != nil {
			panic("failed to read input")
		}
		if bytes[0] == '\r' {
			continue
		} else if bytes[0] == '\n' {
			break
		}
		res = append(res, bytes[0])
	}
	return string(res)
}
