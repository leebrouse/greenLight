package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/leebrouse/greenLight/internal/data"
	"github.com/leebrouse/greenLight/internal/validator"
)

// Generate a newauthentication token
func (app *application) createAuthenticationTokenHandler(w http.ResponseWriter, r *http.Request) {
	//input struct
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	//read json data from the input
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	//new validator
	v := validator.New()

	//check the  format of the email and password
	data.ValidateEmail(v, input.Email)
	data.ValidatePassword(v, input.Password)

	//throw the err if validator have
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	//from the email get the user
	user, err := app.models.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	//match the password
	match, err := user.Password.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}

	//new token
	token, err := app.models.Tokens.New(user.ID, 24*time.Hour, data.ScopeAuthentication)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	//write json message
	err = app.writeJSON(w, http.StatusCreated, envelope{"authentication_token": token}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

}
