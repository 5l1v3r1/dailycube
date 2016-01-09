package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

var AdminPassword string
var Store = sessions.NewCookieStore(securecookie.GenerateRandomKey(16),
	securecookie.GenerateRandomKey(16))
var MainManager Manager
var ClientID string
var Secret string
var CallbackURI = "http://localhost:8599/fblogin_done"

func main() {
	go MainManager.BackgroundRoutine()

	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "Usage: dailycube <admin_password> <client ID> <secret> <port>")
		os.Exit(1)
	}

	AdminPassword = os.Args[1]
	ClientID = os.Args[2]
	Secret = os.Args[3]

	if _, err := strconv.Atoi(os.Args[4]); err != nil {
		fmt.Fprintln(os.Stderr, "Invalid port:", os.Args[2])
		os.Exit(1)
	}

	http.HandleFunc("/login", HandleLogin)
	http.HandleFunc("/fblogin", HandleFBLogin)
	http.HandleFunc("/fblogin_done", HandleFBLoginDone)
	http.HandleFunc("/setgroup", HandleSetGroup)
	http.HandleFunc("/reset", HandleReset)
	http.HandleFunc("/", HandleRoot)

	http.ListenAndServe(":"+os.Args[4], nil)
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	DisableCache(w)
	if r.Method == "POST" && r.FormValue("failed") == "" {
		password := r.PostFormValue("password")
		if password == AdminPassword {
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
	DisableCache(w)
	if !IsAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}
	fbConfig := &oauth2.Config{
		ClientID:     ClientID, // change this to yours
		ClientSecret: Secret,
		RedirectURL:  CallbackURI,
		Scopes:       []string{"publish_actions", "user_managed_groups"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.facebook.com/dialog/oauth",
			TokenURL: "https://graph.facebook.com/oauth/access_token",
		},
	}
	http.Redirect(w, r, fbConfig.AuthCodeURL(""), http.StatusTemporaryRedirect)
}

func HandleFBLoginDone(w http.ResponseWriter, r *http.Request) {
	DisableCache(w)
	if !IsAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	code := r.FormValue("code")
	if err := SetupAccessToken(code); err != nil {
		http.Redirect(w, r, "/fb_error.html", http.StatusTemporaryRedirect)
	} else {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	}
}

func HandleSetGroup(w http.ResponseWriter, r *http.Request) {
	DisableCache(w)
	if !IsAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}
	if err := MainManager.SetGroup(r.FormValue("groupURL")); err != nil {
		http.Redirect(w, r, "/group_error.html", http.StatusTemporaryRedirect)
	} else {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	}
}

func HandleReset(w http.ResponseWriter, r *http.Request) {
	DisableCache(w)
	if !IsAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}
	MainManager.Reset()
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func HandleRoot(w http.ResponseWriter, r *http.Request) {
	DisableCache(w)

	if r.URL.Path == "/" {
		if !IsAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		} else if MainManager.NeedFB() {
			http.ServeFile(w, r, "assets/need_fb.html")
		} else if MainManager.NeedGroup() {
			http.ServeFile(w, r, "assets/need_group.html")
		} else {
			contents, _ := ioutil.ReadFile("assets/running.html")
			page := strings.Replace(string(contents), "%DAYS%",
				fmt.Sprintf("%f", MainManager.DaysRemaining()), 1)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(page))
		}
		return
	}
	fixedPath := strings.Replace(r.URL.Path, "/", "", -1)
	http.ServeFile(w, r, "assets/"+fixedPath)
}

func IsAuthenticated(r *http.Request) bool {
	s, _ := Store.Get(r, "sessid")
	ok, val := s.Values["authenticated"].(bool)
	return ok && val
}

func SetupAccessToken(code string) error {
	response, err := http.Get("https://graph.facebook.com/oauth/access_token?client_id=" +
		ClientID + "&redirect_uri=" + CallbackURI +
		"&client_secret=" + Secret + "&code=" + code)

	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(response.Body)
	response.Body.Close()

	responseInfo, err := url.ParseQuery(string(body))
	if err != nil {
		return err
	}

	expires, err := strconv.Atoi(responseInfo.Get("expires"))
	if err != nil {
		return err
	}

	MainManager.SetAccessToken(responseInfo.Get("access_token"), expires)
	return nil
}

func DisableCache(w http.ResponseWriter) {
	w.Header().Set("cache-control", "private, no-cache, must-revalidate, no-store")
	w.Header().Set("pragma", "no-cache")
	w.Header().Set("expires", "-1")
}
