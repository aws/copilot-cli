package main

import (
	"log"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
)

var magicWords string = os.Getenv("MAGIC_WORDS")

// HealthCheck just returns true if the service is up.
func HealthCheck(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("🚑 healthcheck ok!")
	w.WriteHeader(http.StatusOK)
}

// GetMagicWords returns the environment variable passed in by the arg override
func GetMagicWords(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("Get Succeeded")
	w.WriteHeader(http.StatusOK)
	log.Println(magicWords)
	w.Write([]byte(magicWords))
}

// ServiceDiscoveryGet just returns true no matter what
func ServiceDiscoveryGet(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("Get on ServiceDiscovery endpoint Succeeded")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("back-end-service-discovery"))
}

func main() {
	router := httprouter.New()
	router.GET("/magicwords", GetMagicWords)

	// Health Check
	router.GET("/healthcheck", HealthCheck)

	log.Fatal(http.ListenAndServe(":80", router))
}
