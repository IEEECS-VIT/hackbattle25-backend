package controllers

import (
	"net/http"
)

func Signup(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Signup"))
}

func Login(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Login"))
}
