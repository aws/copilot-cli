package main

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// SimpleGet just returns true no matter what
func SimpleGet(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("Get Succeeded")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("www"))
}

func main() {
	router := httprouter.New()
	router.GET("/www/", SimpleGet)
	log.Fatal(http.ListenAndServe(":80", router))
}
