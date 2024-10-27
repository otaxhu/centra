# Centralized HTTP Error Handling

<div>
  <a href="https://pkg.go.dev/github.com/otaxhu/centra">
    <img src="https://pkg.go.dev/badge/github.com/otaxhu/centra" alt="Go Reference">
  </a>
  <a href="https://coveralls.io/github/otaxhu/centra?branch=main">
    <img src="https://coveralls.io/repos/github/otaxhu/centra/badge.svg?branch=main" alt="Coverage Status">
  </a>
  <a href="https://goreportcard.com/report/github.com/otaxhu/centra">
    <img src="https://goreportcard.com/badge/github.com/otaxhu/centra" alt="Go Report Card">
  </a>
  <a href="https://github.com/otaxhu/centra/actions/workflows/ci.yml">
    <img src="https://github.com/otaxhu/centra/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI Status">
  </a>
</div>

Tired of writing the following code?:
```go
func GetUser(w http.ResponseWriter, r *http.Request) {
    user, err := userService.GetUser()
    if errors.Is(err, userService.ErrNotFound) {
        handleUserNotFound(w, r, err)
        return
    } else if errors.Is(err, userService.ErrBadSearchParameters) {
        handleBadSearchParameters(w, r, err)
        return
    }

    // Return user and OK response to client
}

func PostUser(w http.ResponseWriter, r *http.Request) {
    var user models.User
    err := userService.PostUser(user)
    if errors.Is(err, userService.ErrAlreadyExist) {
        handleUserAlreadyExist(w, r, err)
        return
    } else if errors.Is(err, userService.ErrBadSearchParameters) {
        handleUserBadSearchParameters(w, r, err)
        return
    }

    // Return OK response to client
}
```

Clearly in the previous example there is a duplication of code, this duplication may not be a problem in small APIs, but when you start adding more services, this code will not scale fine.

With Centra as your HTTP Error handler, the previous example can be replaced with:
```go
func GetUser(w http.ResponseWriter, r *http.Request) {
    user, err := userService.GetUser()
    if err != nil {
        centra.Error(w, r, err)
        return
    }

    // Return user and OK response to client
}

func PostUser(w http.ResponseWriter, r *http.Request) {
    var user models.User
    err := userService.PostUser(user)
    if err != nil {
        centra.Error(w, r, err)
        return
    }
}
```

You will need to register HTTP handlers for each of the errors of your services:
```go
func main() {
    mux := chi.NewMux()

    errMux := centra.NewMux()

    errMux.Handle(userService.ErrNotFound, func(w http.ResponseWriter, r *http.Request, err error) {
        w.WriteHeader(http.StatusNotFound)
    })

    errMux.Handle(userService.ErrAlreadyExist, func(w http.ResponseWriter, r *http.Request, err error) {
        w.WriteHeader(http.StatusConflict)
    })

    errMux.Handle(userService.ErrBadSearchParameters, func(w http.ResponseWriter, r *http.Request, err error) {
        w.WriteHeader(http.StatusBadRequest)
    })

    // Optionally you can set unknown handler, by default is DefaultUnknownHandler,
    // which just sends "Internal Server Error" string with "text/html" Content-Type
    errMux.HandleUnknown(func(w http.ResponseWriter, r *http.Request, err error) {
        w.WriteHeader(http.StatusInternalServerError)
    })

    // Build method returns the error handler as a middleware function, compatible with
    // Chi router
    errMw := errMux.Build()
    mux.Use(errMw)

    mux.Get("/users", GetUser)
    mux.Post("/users", PostUser)

    http.ListenAndServe(":8080", mux)
}
```

If you are using dinamically generated errors, It's recommended you use `fmt.Errorf()` with `%w` format placeholder to wrap a statically generated error, for example:
```go
// service/user.go
func GetUser() (*models.User, error) {
    // There is an error, you'd like to generate a dynamic error
    return nil, fmt.Errorf("userService.GetUser: bad search parameters: error %w", ErrBadSearchParameters)
}

// controllers/user.go
func GetUser(w http.ResponseWriter, r *http.Request) {
    user, err := userService.GetUser()
    if err != nil {
        // The library will know that err is a ErrBadSearchParameters
        centra.Error(w, r, err)
        return
    }
}
```

The library will know that the underlying error is `ErrBadSearchParameters` and will match succesfully with the registered error handler.

NOTE: library will not unwrap the error, it will pass it to the error handler just as it is.
