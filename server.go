/*
 MIT License

 Copyright (c) 2018 Max Kuznetsov <syhpoon@syhpoon.ca>

 Permission is hereby granted, free of charge, to any person obtaining a copy
 of this software and associated documentation files (the "Software"), to deal
 in the Software without restriction, including without limitation the rights
 to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 copies of the Software, and to permit persons to whom the Software is
 furnished to do so, subject to the following conditions:

 The above copyright notice and this permission notice shall be included in all
 copies or substantial portions of the Software.

 THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 SOFTWARE.
*/

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
)

type Server struct {
	router   *mux.Router
	server   http.Server
	listener net.Listener
	ctx      context.Context
	sessions map[string]chan *BroMessage
	sync.RWMutex
}

func NewServer(listener net.Listener, ctx context.Context) *Server {
	router := mux.NewRouter()

	return &Server{
		router: router,
		server: http.Server{
			Handler: router,
		},
		listener: listener,
		sessions: map[string]chan *BroMessage{},
		ctx:      ctx,
	}
}

func (s *Server) Run(wg *sync.WaitGroup) {
	defer wg.Done()

	// API endpoints
	s.setupHandlers()

	go func() {
		<-s.ctx.Done()
		_ = s.server.Shutdown(s.ctx)

		wg.Done()
	}()

	log.Printf("Starting bro server at %s", s.listener.Addr().String())

	var err error

	if err = s.server.Serve(s.listener); err != nil {
		log.Printf("Error running bro server: %+v", err)
	}

	return
}

func (s *Server) setupHandlers() {
	// POST /v1/bro - Post a bro message
	s.router.HandleFunc("/v1/bro", s.postHandler).
		Methods(http.MethodPost)

	// GET /v1/poll - Poll for new bro messages
	s.router.HandleFunc("/v1/poll/{id}", s.pollHandler).
		Methods(http.MethodGet)
}

func (s *Server) postHandler(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	body, err := ioutil.ReadAll(req.Body)

	if err != nil {
		log.Printf("Error reading request body: %s", err)

		SendHttpResponse(w, http.StatusBadRequest,
			"Error reading request body: %s", err)

		return
	}

	msg := BroMessage{}

	if err = json.Unmarshal(body, &msg); err != nil {
		log.Printf("Error decoding request body: %s", err)

		SendHttpResponse(w, http.StatusBadRequest,
			"Error decoding request body: %s", err)

		return
	}

	s.Lock()
	for id, ch := range s.sessions {
		if id != msg.From {
			select {
			case ch <- &msg:
			default:
			}
		}
	}
	s.Unlock()

	SendHttpResponse(w, http.StatusOK, nil, "")
}

func (s *Server) pollHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]

	s.RLock()
	if _, ok := s.sessions[id]; ok {
		SendHttpResponse(w, http.StatusConflict,
			nil, fmt.Sprintf("Session id %s already taken", id))

		s.RUnlock()
		return
	}
	s.RUnlock()

	ch := make(chan *BroMessage, 1)

	s.Lock()
	s.sessions[id] = ch
	s.Unlock()

	defer func() {
		s.Lock()
		delete(s.sessions, id)
		s.Unlock()
	}()

	f, ok := w.(http.Flusher)

	if !ok {
		SendHttpResponse(w, http.StatusInternalServerError,
			nil, "Request cannot be casted to Flusher")

		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Access-Control-Allow-Origin", "*")

	for {
		select {
		case <-req.Context().Done():
			return
		case msg := <-ch:
			js, _ := json.MarshalIndent(msg, "", "   ")

			w.Write(js)
			f.Flush()
		}
	}
}
