package main

import (
	"context"
	"fmt"
	"lorraxs/fivem_cdn_server/config"
	"lorraxs/fivem_cdn_server/controllers"
	"net/http"
	"os"

	"github.com/charmbracelet/log"
	"github.com/gorilla/handlers"
)

func main() {
	ctx := context.Background()
	config := config.GetConfig()
	if _, err := os.Stat(config.App.UploadPath); os.IsNotExist(err) {
		err := os.Mkdir(config.App.UploadPath, 0755)
		if err != nil {
			log.Error("Error creating upload path: " + err.Error())
			panic(err)
		}
	}
	fmt.Printf("%+v\n", config)
	router := getRouter()

	/* controllers.NewBanListController().Init(ctx, mongoClient, router)
	controllers.NewLauncherController().Init(ctx, mongoClient, router)
	controllers.NewChecksumController().Init(ctx, mongoClient, router) */

	controllers.NewUploadController().Init(ctx, router)

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf(("%s\n"), r.Header)
		fmt.Fprint(w, "Hello, World!")
	})

	loggedRouter := handlers.LoggingHandler(os.Stdout, router)
	log.Info("Starting server on port " + config.Http.Port)
	http.ListenAndServe(fmt.Sprintf(":%s", config.Http.Port), loggedRouter)
}
