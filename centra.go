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
	"net/http"
	"strconv"
)

type Mux struct {
	handlers map[error]func(w http.ResponseWriter, r *http.Request, err error)
}

func NewMux() *Mux {
	return &Mux{
		handlers: map[error]func(w http.ResponseWriter, r *http.Request, err error){
			nil: DefaultUnknownHandler,
		},
	}
}

type keyHandlersType struct{}

var keyHandlers keyHandlersType

func (m *Mux) Build() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), keyHandlers, m.handlers))

			next.ServeHTTP(w, r)
		})
	}
}

func (m *Mux) Handle(err error, handler func(w http.ResponseWriter, r *http.Request, err error)) {
	if err == nil {
		panic("centra: err must not be nil")
	}

	if handler == nil {
		panic("centra: handler must not be nil")
	}

	_, ok := m.handlers[err]
	if ok {
		panic(fmt.Sprintf("centra: cannot register more than one handler for the same error: %s", err.Error()))
	}

	m.handlers[err] = handler
}

func (m *Mux) HandleUnknown(handler func(w http.ResponseWriter, r *http.Request, err error)) {
	if handler == nil {
		panic("centra: handler must not be nil")
	}
	m.handlers[nil] = handler
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
	handlers := getHandlers(r)
	if handlers == nil {
		// Mux has not been initialized, should we panic or call default handle unknown?
		DefaultUnknownHandler(w, r, err)
		return
	}
	if err == nil {
		// As special case, if err is nil, call unknown handler
		handlers[nil](w, r, err)
		return
	}
	for targetError, handler := range handlers {
		if errors.Is(err, targetError) {
			handler(w, r, err)
			return
		}
	}
}

func DefaultUnknownHandler(w http.ResponseWriter, r *http.Request, err error) {
	response := "<h1>Internal Server Error</h1>"

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Length", strconv.Itoa(len(response)))

	w.WriteHeader(http.StatusInternalServerError)

	fmt.Fprint(w, response)
}

func getHandlers(r *http.Request) map[error]func(w http.ResponseWriter, r *http.Request, err error) {
	return r.Context().Value(keyHandlers).(map[error]func(w http.ResponseWriter, r *http.Request, err error))
}
