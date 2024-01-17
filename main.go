package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"os"

	firebase "firebase.google.com/go/v4"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)
	

func main () { 
	////////
	// flags
	////////
	devFlag := flag.Bool("dev", false, "Enable development mode. Reads .env.dev instead of .env")
	flag.Parse()

	////////////////
	// env variables
	////////////////
	envFile := ".env"
	if(*devFlag) {
		envFile += ".dev"
	}

	if err := godotenv.Load(envFile); err != nil {
		log.Fatal(err)
	}

	////////////////
	// init firebase
	////////////////
	serviceAccountPathEnvVar, serviceAccountPathEnvVarExists := os.LookupEnv("SERVICE_ACCOUNT_PATH")
	if(!serviceAccountPathEnvVarExists || serviceAccountPathEnvVar == "") { 
		log.Fatal(errors.New("'SERVICE_ACCOUNT_PATH' environment variable undefined"))
	}

	opt := option.WithCredentialsFile(serviceAccountPathEnvVar)
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
	  log.Fatalf("error initializing firebase: %v\n", err)
	}

	// init firebase authClient for user validation
	// todo: integrate this bullshit into middleware 
	// authClient, err := app.Auth(ctx)
	// if err != nil {
    //     log.Fatalf("error getting firebase Auth client: %v\n", err)
	// }

	// init firebase firestore db client 
	// todo: actual repo layer 
	dbClient, err := app.Firestore(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	defer dbClient.Close()

	

	////////////
	// init gin 
	////////////
	if(!*devFlag) {
		gin.SetMode(gin.ReleaseMode)
	}

	logFile, _ := os.Create("gin.log")
	gin.DefaultWriter = io.MultiWriter(logFile, os.Stdout)

	router := gin.Default()

	////////////////
	// define routes
	////////////////



	//////////
	// run gin
	//////////
	pathEnvVar, pathEnvVarExists := os.LookupEnv("GIN_PATH")
	if(!pathEnvVarExists || pathEnvVar == "") { 
		log.Fatal(errors.New("'GIN_PATH' variable undefined"))
	}

	if err := router.Run(pathEnvVar); err != nil { 
		log.Fatal(err)
	}
}