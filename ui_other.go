//go:build !windows

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// Development stand-in for the window: serves the same page over http and relays the same RPC,
// so the UI and the Go handlers can be tested in a normal browser. Not part of the Windows build.
func runGUI(env *Env) {
	addr := os.Getenv("INSTALLER_UI_ADDR")
	if addr == "" {
		addr = "127.0.0.1:18080"
	}
	var mu sync.Mutex
	clients := map[chan string]bool{}
	var backlog []string // answers produced before the page's event stream connected
	ui := &UI{env: env}
	ui.eval = func(js string) {
		mu.Lock()
		defer mu.Unlock()
		if len(clients) == 0 {
			backlog = append(backlog, js)
			return
		}
		for c := range clients {
			select {
			case c <- js:
			default:
			}
		}
	}
	ui.quit = func() { fmt.Println("quit requested") }
	dev := `<script>window.__ipSend=function(r){fetch('/rpc',{method:'POST',body:r})};` +
		`new EventSource('/events').onmessage=function(e){(0,eval)(e.data)};</script>`
	page := strings.Replace(indexHTML, "<!--DEV-->", dev, 1)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, page)
	})
	http.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		ui.handle(string(b))
	})
	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		c := make(chan string, 256)
		mu.Lock()
		clients[c] = true
		for _, js := range backlog {
			c <- js
		}
		backlog = nil
		mu.Unlock()
		defer func() { mu.Lock(); delete(clients, c); mu.Unlock() }()
		fl := w.(http.Flusher)
		fmt.Fprint(w, ": hi\n\n")
		fl.Flush()
		for {
			select {
			case js := <-c:
				for _, line := range strings.Split(js, "\n") {
					fmt.Fprintf(w, "data: %s\n", line)
				}
				fmt.Fprint(w, "\n")
				fl.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
	fmt.Println("dev ui on http://" + addr)
	_ = http.ListenAndServe(addr, nil)
}

