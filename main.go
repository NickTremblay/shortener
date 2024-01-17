package main

import (
	"errors"
	"flag"
	"io"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)
	

func main () { 
	// flags
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

	////////////
	// init gin 
	////////////
	if(!*devFlag) {
		gin.SetMode(gin.ReleaseMode)
	}

	logFile, _ := os.Create("gin.log")
	gin.DefaultWriter = io.MultiWriter(logFile, os.Stdout)
	
	pathEnvVar, pathEnvVarExists := os.LookupEnv("GIN_PATH")
	if(!pathEnvVarExists || pathEnvVar == "") { 
		log.Fatal(errors.New("'GIN_PATH' variable undefined"))
	}

	router := gin.Default()
	if err := router.Run(pathEnvVar); err != nil { 
		log.Fatal(err)
	}
}