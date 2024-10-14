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
	fnErrorFactory := func(message string) func(w http.ResponseWriter, r *http.Request, err error) {
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
		ErrorsToHandle map[error]func(w http.ResponseWriter, r *http.Request, err error)

		ExpectedBuf string
	}{
		"Error": {
			FinalHandler: fnFailingFinalFactory(errString("Fail")),
			ErrorsToHandle: map[error]func(w http.ResponseWriter, r *http.Request, err error){
				errString("Fail"): fnErrorFactory("1"),
			},

			ExpectedBuf: "1",
		},
		"Error_Wrapped": {
			FinalHandler: fnFailingFinalFactory(errStringWrapped("Fail")),
			ErrorsToHandle: map[error]func(w http.ResponseWriter, r *http.Request, err error){

				errString("Fail_UNWRAP"): fnErrorFactory("1"),
			},

			ExpectedBuf: "1",
		},
		"Ok": {
			FinalHandler: fnOkFinalFactory("1"),
			ErrorsToHandle: map[error]func(w http.ResponseWriter, r *http.Request, err error){
				errString("Unreachable"): fnErrorFactory("2"),
			},

			ExpectedBuf: "1",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			errMux := NewMux()
			for k, v := range tc.ErrorsToHandle {
				errMux.Handle(k, v)
			}

			req := httptest.NewRequest("", "/", nil)

			recorder := httptest.NewRecorder()

			errMw := errMux.Build()
			errMw(tc.FinalHandler).ServeHTTP(recorder, req)

			if tc.ExpectedBuf != recorder.Body.String() {
				t.Fatalf("expected %s, got %s", tc.ExpectedBuf, recorder.Body.String())
			}
		})
	}
}
