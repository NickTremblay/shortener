package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		log.Fatalf("error loading env variables: %v\n", err)
	}

	// todo: unbad env var lookup. flag system maybe? 

	linkIdLengthEnvVar, linkIdLengthEnvVarExists := os.LookupEnv("LINK_ID_LENGTH")
	if(!linkIdLengthEnvVarExists || linkIdLengthEnvVar == "") { 
		log.Println("'LINK_ID_LENGTH' environment variable undefined, defaulting to 6")
		linkIdLengthEnvVar = "6"
	}
	linkIdLength, err := strconv.Atoi(linkIdLengthEnvVar)
	if(err != nil) { 
		log.Printf("error parsing LINK_ID_LENGTH env var, defaulting to 6: %v\n", err)
		linkIdLength = 6
	}

	////////////////
	// init firebase
	////////////////
	// todo: iterate through configured list of expected collections and create them if they do not exist 
	serviceAccountPathEnvVar, serviceAccountPathEnvVarExists := os.LookupEnv("SERVICE_ACCOUNT_PATH")
	if(!serviceAccountPathEnvVarExists || serviceAccountPathEnvVar == "") { 
		log.Fatal(errors.New("'SERVICE_ACCOUNT_PATH' environment variable undefined"))
	}

	opt := option.WithCredentialsFile(serviceAccountPathEnvVar)
	// Firebase context 
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

	router.POST("/shorten", func(c *gin.Context) {
		// Todo: restrict this route to requests w auth token w verified uri email
		// token := c.Request.Header["Token"]

		newId, err := generateLinkId(uint(linkIdLength), &ctx, dbClient)
		if err != nil { 
			log.Printf("error generating link id: %s", err)
			c.String(http.StatusInternalServerError, err.Error())
		}

		// Todo: sanitize and validate user input
		url := ""
		var json ShortenRequestBody
        if err = c.BindJSON(&json); err != nil {
            log.Printf("error binding ShortenRequestBody: %s", err)
			c.String(http.StatusInternalServerError, err.Error())
        }

		url = json.Url 

		link := Link {
			Id: newId,                    
			Url: url,                     
			Author_Address: c.ClientIP(), 
			Author_Email: "place@hold.er", // get from firebase auth
		}

		_, err = dbClient.Collection("links").Doc(newId).Set(ctx, link)
        if err != nil {
			log.Printf("error creating new document in collection 'links': %s", err.Error())
			c.String(http.StatusInternalServerError, err.Error())
        }

		//todo: unbad
		c.String(http.StatusOK, "http://" + c.Request.URL.Host + "/" + newId)
	})

	// keep this route last, or else shortened routes will override matching routes 
	router.GET("/:linkId", func(c *gin.Context) {
        linkId := c.Param("linkId") 
        linkDoc, err := dbClient.Collection("links").Doc(linkId).Get(ctx)
		if(err != nil) { 
			// todo: 401 html 
			c.String(http.StatusInternalServerError, err.Error())
			log.Printf("error searching for link: %v\n", err)
		} 

    	if(linkDoc.Data() != nil){ 
			expandedUrl, err := linkDoc.DataAt("url")
			if(err != nil) { 
				c.String(http.StatusInternalServerError, err.Error())
				log.Fatalf("error binding expandedUrl: %v\n", err)
			}

			c.Redirect(http.StatusPermanentRedirect, expandedUrl.(string))
		} else { 
		// todo: 404 html 
		c.String(http.StatusNotFound, "not found")
	   }
    })

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

///////////////
// define types 
///////////////

type Link struct {
	Id                string    `json:"id" firestore:"id"`
	Url               string    `json:"url" firestore:"url"`
	Author_Address    string    `json:"author_address" firestore:"author_address"`
	Author_Email      string    `json:"author_email" firestore:"author_email"`
	Created           time.Time `json:"created" firestore:"created,serverTimestamp"`
}

type ShortenRequestBody struct { 
	Url string `json:"url" binding:"required"`
}

///////////////////
// define functions 
///////////////////

// Generate unique id for a Link. 
func generateLinkId(n uint, ctx *context.Context, dbClient *firestore.Client) (string, error) { 	
	newId, err := generateLinkToken(n)
	if err != nil { 
		return "", err
	}

	newIdUnique := false

	linkDocRef := dbClient.Collection("links").Doc(newId)
    _, err = linkDocRef.Get(*ctx)
    if err != nil {
        if status.Code(err) == codes.NotFound {
            newIdUnique = true
        } else {
            return "", err
        }
    }

	if newIdUnique { 
		return newId, nil
	} else { 
		return generateLinkId(n, ctx, dbClient)
	}
}

// Generate potentially non-unique id for a Link.
func generateLinkToken(n uint) (string, error) { 
	if(n == 0){ 
		return "", nil
	} else { 
		symbols := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ12345678910"

		// b is a random byte from the random reader
		b := []byte{0}
		if _, err := rand.Reader.Read(b); err != nil {
			return "", err
		}
	
		// random float on (0, 1]
		x := float64(b[0]) / 255
	
		newSymbolIndex := int64(math.Floor(x * float64(len(symbols) - 1))) 
		newSymbol := string(symbols[newSymbolIndex])

		if rest, err := generateLinkToken(n - 1); err != nil { 
			return "", err
		}else { 
			return newSymbol + rest, nil; 
		}		
	}
}

