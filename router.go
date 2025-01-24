package main

import (
	"sync"

	"github.com/charmbracelet/log"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var lock = &sync.Mutex{}
var router *mux.Router

func getRouter() *mux.Router {
	if router == nil {
		lock.Lock()
		defer lock.Unlock()
		router = mux.NewRouter()
		router.Use(handlers.ProxyHeaders)
		log.Info("Router created")
	}
	return router
}
