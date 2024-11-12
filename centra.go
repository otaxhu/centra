// Copyright 2024 Oscar Pernia
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package centra

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
)

type handlerStruct struct {
	err     error
	handler ErrorHandlerFunc
}

// Multiplexer error handler, multiplexes a call to [Error] to the registered error handler,
// if error is not found, then a call to the registered UnknownHandler is made.
type Mux struct {
	handlersStack []handlerStruct
	mu            sync.RWMutex
}

// Returns a new Mux with UnknownHandler set to DefaultUnknownError.
func NewMux() *Mux {
	return &Mux{
		handlersStack: []handlerStruct{
			{
				err:     nil,
				handler: DefaultUnknownHandler,
			},
		},
	}
}

// Function type to handle errors
type ErrorHandlerFunc func(w http.ResponseWriter, r *http.Request, err error)

type keyContext struct{}

// Middleware handler, compatible with Chi router, changes the request's context and adds
// the error handlers to it.
func (m *Mux) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), keyContext{}, m))

		next.ServeHTTP(w, r)
	})
}

// Sets handler to handle err when a call to Error(w, r, errOrWrappedErr) is made in the context
// of a http request.
func (m *Mux) Handle(err error, handler ErrorHandlerFunc) {
	if err == nil {
		panic("centra: err must not be nil")
	}

	if handler == nil {
		panic("centra: handler must not be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlersStack = append(m.handlersStack, handlerStruct{
		err:     err,
		handler: handler,
	})
}

// Sets handler to handle unknown errors when a call to Error(w, r, err) doesn't find a registered
// error handler for err.
func (m *Mux) UnknownHandler(handler ErrorHandlerFunc) {
	if handler == nil {
		panic("centra: handler must not be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlersStack[0] = handlerStruct{
		err:     nil,
		handler: handler,
	}
}

// Returns the registered UnknownHandler, if [Mux.UnknownHandler] has not been called yet,
// by default it is [DefaultUnknownHandler]
func (m *Mux) GetUnknownHandler() ErrorHandlerFunc {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.handlersStack[0].handler
}

// Error search for registered error handlers to handle err, if no error handler is found, then
// it calls the registered UnknownHandler
func Error(w http.ResponseWriter, r *http.Request, err error) {
	mux := getMux(r)
	if mux == nil {
		// TODO: panic or DefaultUnknownHandler?
		//
		// For now we are panicking, since this should be a invalid state for the library,
		// and calling Default may not be desired behaviour.
		panic("centra: Mux has not been initialized, cannot call Error() for this request")
	}
	mux.mu.RLock()
	defer mux.mu.RUnlock()
	if err == nil {
		// as a special case, if err is nil, call unknown handler
		mux.handlersStack[0].handler(w, r, err)
		return
	}
	for i := len(mux.handlersStack) - 1; i >= 1; i-- {
		h := mux.handlersStack[i]
		if errors.Is(err, h.err) {
			h.handler(w, r, err)
			return
		}
	}

	// if err is not registered, then call unknown error handler
	mux.handlersStack[0].handler(w, r, err)
}

// Default error handler for unknown errors
//
// Writes string "<h1>Internal Server Error</h1>" to w, sets Content-Type to "text/html"
// and writes status code 500
func DefaultUnknownHandler(w http.ResponseWriter, r *http.Request, err error) {
	response := "<h1>Internal Server Error</h1>"

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Length", strconv.Itoa(len(response)))

	w.WriteHeader(http.StatusInternalServerError)

	w.Write([]byte(response))
}

func getMux(r *http.Request) *Mux {
	m, _ := r.Context().Value(keyContext{}).(*Mux)
	return m
}
