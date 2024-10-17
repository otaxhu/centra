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
	"fmt"
	"log"
	"net/http"
	"strconv"
)

// Multiplexer error handler, multiplexes a call to [Error] to the registered error handler,
// if error is not found, then a call to the registered UnknownHandler is made.
type Mux struct {
	handlers map[error]ErrorHandlerFunc
	options  Options
}

// Returns a new Mux with UnknownHandler set to DefaultUnknownError.
func NewMux() *Mux {
	return &Mux{
		handlers: map[error]ErrorHandlerFunc{
			nil: DefaultUnknownHandler,
		},
		options: Options{
			Debug:  false,
			Logger: log.Default(),
		},
	}
}

// Options to be passed to Mux
type Options struct {
	Debug  bool
	Logger *log.Logger
}

// Function type to handle errors
type ErrorHandlerFunc func(w http.ResponseWriter, r *http.Request, err error)

type keyHandlersType struct{}

var keyHandlers keyHandlersType

// Returns a middleware compatible with Chi router, that changes the request's context and adds
// the error handlers to it.
func (m *Mux) Build() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), keyHandlers, m))

			next.ServeHTTP(w, r)
		})
	}
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

	m.handlers[err] = handler
}

// Sets handler to handle unknown errors when a call to Error(w, r, err) doesn't find a registered
// error handler for err.
func (m *Mux) UnknownHandler(handler ErrorHandlerFunc) {
	if handler == nil {
		panic("centra: handler must not be nil")
	}
	m.handlers[nil] = handler
}

// Sets options to Mux
func (m *Mux) Options(options Options) {
	if options.Logger == nil {
		options.Logger = log.Default()
	}
	m.options = options
}

func (m *Mux) log(err error, found bool) {
	if !m.options.Debug {
		return
	}

	prefix := "centra: DEBUG: "

	if !found {
		m.options.Logger.Printf(prefix+"Unknown handler has been called for the following error `%s`", err.Error())
		return
	}

	if err == nil {
		m.options.Logger.Println(prefix + "Unknown handler has been called, err is <nil>")
		return
	}

	m.options.Logger.Printf(prefix+"Handler for error `%s` has been called", err.Error())
}

// Error search for registered error handlers to handle err, if no error handler is found, then
// it calls the registered UnknownHandler
func Error(w http.ResponseWriter, r *http.Request, err error) {
	mux := getMux(r)
	if mux == nil {
		// Mux has not been initialized, should we panic or call default handle unknown?
		DefaultUnknownHandler(w, r, err)
		return
	}
	if handler, ok := mux.handlers[err]; ok {
		mux.log(err, true)
		// As special case, if err is nil, call unknown handler
		handler(w, r, err)
		return
	}
	for targetError, handler := range mux.handlers {
		if errors.Is(err, targetError) {
			mux.log(err, true)
			handler(w, r, err)
			return
		}
	}

	mux.log(err, false)
	// if err is not registered, then call unknown error handler
	mux.handlers[nil](w, r, err)
}

// Default error handler for unknown errors
func DefaultUnknownHandler(w http.ResponseWriter, r *http.Request, err error) {
	response := "<h1>Internal Server Error</h1>"

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Length", strconv.Itoa(len(response)))

	w.WriteHeader(http.StatusInternalServerError)

	fmt.Fprint(w, response)
}

func getMux(r *http.Request) *Mux {
	return r.Context().Value(keyHandlers).(*Mux)
}
