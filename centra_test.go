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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type errString string

type errStringWrapped string

func (e errString) Error() string {
	return string(e)
}

func (ew errStringWrapped) Error() string {
	return string(ew)
}

func (ew errStringWrapped) Unwrap() error {
	return errString(ew + "_UNWRAP")
}

func TestHandleAndError(t *testing.T) {
	fnErrorFactory := func(message string) ErrorHandlerFunc {
		return func(w http.ResponseWriter, r *http.Request, err error) {
			io.WriteString(w, message)
		}
	}

	fnFailingFinalFactory := func(err error) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			Error(w, r, err)
		}
	}

	fnOkFinalFactory := func(message string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, message)
		}
	}

	testCases := map[string]struct {
		FinalHandler   http.HandlerFunc
		ErrorsToHandle map[error]ErrorHandlerFunc

		ExpectedBuf   string
		ExpectedPanic bool
	}{
		"Error": {
			FinalHandler: fnFailingFinalFactory(errString("Fail")),
			ErrorsToHandle: map[error]ErrorHandlerFunc{
				errString("Fail"): fnErrorFactory("1"),
			},

			ExpectedBuf:   "1",
			ExpectedPanic: false,
		},
		"Error_Wrapped": {
			FinalHandler: fnFailingFinalFactory(errStringWrapped("Fail")),
			ErrorsToHandle: map[error]ErrorHandlerFunc{

				errString("Fail_UNWRAP"): fnErrorFactory("1"),
			},

			ExpectedBuf:   "1",
			ExpectedPanic: false,
		},
		"Ok": {
			FinalHandler: fnOkFinalFactory("1"),
			ErrorsToHandle: map[error]ErrorHandlerFunc{
				errString("Unreachable"): fnErrorFactory("2"),
			},

			ExpectedBuf:   "1",
			ExpectedPanic: false,
		},
		"Error_Unknown_Default": {
			FinalHandler:   fnFailingFinalFactory(errors.New("Unknown")),
			ErrorsToHandle: map[error]ErrorHandlerFunc{},
			ExpectedBuf:    "<h1>Internal Server Error</h1>",
			ExpectedPanic:  false,
		},
		"Error_Unknown_Setted": {
			FinalHandler: fnFailingFinalFactory(errors.New("Unknown")),
			ErrorsToHandle: map[error]ErrorHandlerFunc{
				// Unknown handler
				nil: fnErrorFactory("2"),
			},
			ExpectedBuf:   "2",
			ExpectedPanic: false,
		},
		"Error_Is_Nil": {
			FinalHandler: fnFailingFinalFactory(nil),
			ErrorsToHandle: map[error]ErrorHandlerFunc{
				nil: fnErrorFactory("2"),
			},
			ExpectedBuf:   "2",
			ExpectedPanic: false,
		},
		"Panic_Handler_IsNil": {
			FinalHandler: fnOkFinalFactory("1"),
			ErrorsToHandle: map[error]ErrorHandlerFunc{
				errString("err"): nil,
			},
			ExpectedBuf:   "",
			ExpectedPanic: true,
		},
		"Panic_Unknown_Handler_IsNil": {
			FinalHandler: fnOkFinalFactory("1"),
			ErrorsToHandle: map[error]ErrorHandlerFunc{
				nil: nil,
			},
			ExpectedBuf:   "",
			ExpectedPanic: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tc.ExpectedPanic && r == nil {
					t.Fatalf("expected to panic, did not panic")
				} else if !tc.ExpectedPanic && r != nil {
					t.Fatalf("expected to not panic, did panic: %v", r)
				}
			}()
			errMux := NewMux()
			for k, v := range tc.ErrorsToHandle {
				if k == nil {
					errMux.UnknownHandler(v)
					continue
				}
				errMux.Handle(k, v)
			}

			req := httptest.NewRequest("", "/", nil)

			recorder := httptest.NewRecorder()

			errMux.Handler(tc.FinalHandler).ServeHTTP(recorder, req)

			if tc.ExpectedBuf != recorder.Body.String() {
				t.Fatalf("expected %s, got %s", tc.ExpectedBuf, recorder.Body.String())
			}
		})
	}
}
