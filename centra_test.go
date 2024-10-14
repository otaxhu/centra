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
