package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
)

func signalHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(400)
			return
		}

		fmt.Println(body)
	}
}

func TradesHandler(router *mux.Router) {
	router.HandleFunc("/signal", signalHandler)
}
