package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

var Store = sessions.NewCookieStore(securecookie.GenerateRandomKey(16),
	securecookie.GenerateRandomKey(16))
var GlobalState State
var GlobalConfig *Config

var LastCSRFTokenLock sync.Mutex
var LastCSRFToken string

var PostNowChan chan struct{}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: dailycube <config.json> <port>")
		os.Exit(1)
	}

	var err error
	if GlobalConfig, err = LoadConfig(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "Could not load configuration:", err)
		os.Exit(1)
	}
	if _, err := strconv.Atoi(os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "Invalid port:", os.Args[2])
		os.Exit(1)
	}

	PostNowChan = make(chan struct{}, 1)
	go PostLoop(&GlobalState, GlobalConfig.Location, PostNowChan)

	http.HandleFunc("/login", HandleLogin)
	http.HandleFunc("/fblogin", HandleFBLogin)
	http.HandleFunc("/fblogin_done", HandleFBLoginDone)
	http.HandleFunc("/setgroup", HandleSetGroup)
	http.HandleFunc("/reset", HandleReset)
	http.HandleFunc("/post_now", HandlePostNow)
	http.HandleFunc("/", HandleRoot)

	http.ListenAndServe(":"+os.Args[2], nil)
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	disableCache(w)
	if r.Method == "POST" && r.FormValue("failed") == "" {
		password := r.PostFormValue("password")
		if GlobalConfig.CheckPassword(password) {
			s, _ := Store.Get(r, "sessid")
			s.Values["authenticated"] = true
			s.Save(r, w)
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		} else {
			http.Redirect(w, r, "/login?failed=1", http.StatusTemporaryRedirect)
		}
		return
	}
	http.ServeFile(w, r, "assets/login.html")
}

func HandleFBLogin(w http.ResponseWriter, r *http.Request) {
	disableCache(w)
	if !isAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}
	fbConfig := &oauth2.Config{
		ClientID:     GlobalConfig.ClientID,
		ClientSecret: GlobalConfig.Secret,
		RedirectURL:  GlobalConfig.CallbackURI,
		Scopes:       []string{"publish_actions", "user_managed_groups"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.facebook.com/dialog/oauth",
			TokenURL: "https://graph.facebook.com/oauth/access_token",
		},
	}
	token := hex.EncodeToString(securecookie.GenerateRandomKey(16))
	LastCSRFTokenLock.Lock()
	LastCSRFToken = token
	LastCSRFTokenLock.Unlock()
	http.Redirect(w, r, fbConfig.AuthCodeURL(token), http.StatusTemporaryRedirect)
}

func HandleFBLoginDone(w http.ResponseWriter, r *http.Request) {
	disableCache(w)
	if !isAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	LastCSRFTokenLock.Lock()
	token := LastCSRFToken
	LastCSRFToken = ""
	LastCSRFTokenLock.Unlock()

	if token != r.FormValue("state") {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	code := r.FormValue("code")
	if token, expires, err := requestAccessToken(code); err != nil {
		http.Redirect(w, r, "/fb_error.html", http.StatusTemporaryRedirect)
	} else {
		GlobalState.SetAccessToken(token, expires)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	}
}

func HandleSetGroup(w http.ResponseWriter, r *http.Request) {
	disableCache(w)
	if !isAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}
	if groupID, err := groupURLID(r.FormValue("groupURL")); err != nil {
		http.Redirect(w, r, "/group_error.html", http.StatusTemporaryRedirect)
	} else {
		GlobalState.SetGroupID(groupID)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	}
}

func HandleReset(w http.ResponseWriter, r *http.Request) {
	disableCache(w)
	if !isAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}
	GlobalState.Reset()
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func HandlePostNow(w http.ResponseWriter, r *http.Request) {
	disableCache(w)
	if !isAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}
	select {
	case PostNowChan <- struct{}{}:
	default:
	}
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func HandleRoot(w http.ResponseWriter, r *http.Request) {
	disableCache(w)

	if r.URL.Path == "/" {
		if !isAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		} else if GlobalState.NeedAccessToken() {
			http.ServeFile(w, r, "assets/need_fb.html")
		} else if GlobalState.NeedGroupID() {
			http.ServeFile(w, r, "assets/need_group.html")
		} else {
			contents, _ := ioutil.ReadFile("assets/running.html")
			page := strings.Replace(string(contents), "%DAYS%",
				fmt.Sprintf("%f days", GlobalState.DaysRemaining()), 1)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(page))
		}
		return
	}
	fixedPath := strings.Replace(r.URL.Path, "/", "", -1)
	http.ServeFile(w, r, "assets/"+fixedPath)
}

func isAuthenticated(r *http.Request) bool {
	s, _ := Store.Get(r, "sessid")
	ok, val := s.Values["authenticated"].(bool)
	return ok && val
}

func requestAccessToken(code string) (token string, expires int, err error) {
	response, err := http.Get("https://graph.facebook.com/oauth/access_token?client_id=" +
		GlobalConfig.ClientID + "&redirect_uri=" + GlobalConfig.CallbackURI +
		"&client_secret=" + GlobalConfig.Secret + "&code=" + code)

	if err != nil {
		return "", 0, err
	}

	body, err := ioutil.ReadAll(response.Body)
	response.Body.Close()

	responseInfo, err := url.ParseQuery(string(body))
	if err != nil {
		return "", 0, err
	}

	expires, err = strconv.Atoi(responseInfo.Get("expires"))
	if err != nil {
		return "", 0, err
	}

	return responseInfo.Get("access_token"), expires, nil
}

// disableCache sets headers indicating not to cache the page.
func disableCache(w http.ResponseWriter) {
	w.Header().Set("cache-control", "private, no-cache, must-revalidate, no-store")
	w.Header().Set("pragma", "no-cache")
	w.Header().Set("expires", "-1")
}

// groupURLID extracts the group ID from a group URL.
func groupURLID(u string) (string, error) {
	exp := regexp.MustCompile("https?:\\/\\/(www\\.)?facebook.com\\/groups\\/([0-9]*)\\/?")
	match := exp.FindStringSubmatch(u)
	if len(match) != 3 {
		return "", errors.New("invalid URL")
	}
	return match[2], nil
}
