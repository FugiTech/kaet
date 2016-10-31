package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

func init() {
	http.HandleFunc("/", home)
	http.HandleFunc("/_github", githubWebhook)
}

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome Home")
}

func githubWebhook(w http.ResponseWriter, r *http.Request) {
	event := r.Header.Get("X-GitHub-Event")
	signature := r.Header.Get("X-Hub-Signature")
	if event == "" || signature == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Missing Event or Signature")
		return
	}
	if event != "push" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Invalid event")
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error reading body: %s\n", err)
		return
	}

	var hashMethod string
	p := strings.SplitN(signature, "=", 2)
	hashMethod, signature = p[0], p[1]

	var hasher func() hash.Hash
	switch hashMethod {
	case "sha1":
		hasher = sha1.New
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Invalid Hash Method")
		return
	}

	hmacer := hmac.New(hasher, GITHUB_SECRET)
	hmacer.Write(body)
	computed := hex.EncodeToString(hmacer.Sum(nil))
	if computed != signature {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Invalid signature")
		return
	}

	var data *struct {
		RefName string `json:"ref_name"`
		Ref     string `json:"ref"`
	}
	err = json.Unmarshal(body, &data)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Unparsable body")
		return
	}

	if data.RefName == "" {
		data.RefName = strings.Replace(strings.Replace(data.Ref, "refs/tags/", "", -1), "refs/heads/", "", -1)
	}
	if data.RefName != "master" {
		fmt.Fprintln(w, "Ignoring as branch isn't master")
		return
	}

	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(420)
	}()

	fmt.Fprintln(w, "Updating in 1 second...")
}
