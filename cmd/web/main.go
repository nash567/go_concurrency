package main

import (
	"database/sql"
	"encoding/gob"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"
	_ "github.com/jackc/pgconn"
	_ "github.com/jackc/pgx/v4"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/messageService/data"
)

const webPort = "8085"

func main() {
	db := initDB()

	// create redis session
	session := initSession()

	//create logger
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime|log.Lshortfile)

	//waitgroup
	wg := &sync.WaitGroup{}
	//app config

	app := &Config{
		Session:       session,
		DB:            db,
		wait:          wg,
		InfoLog:       infoLog,
		ErrLog:        errLog,
		Models:        data.New(db),
		ErrorChan:     make(chan error),
		ErrorChanDone: make(chan bool),
	}
	app.serve()
}

func (app *Config) serve() {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", webPort),
		Handler: app.Routes(),
	}
	app.Mailer = app.createMail()
	go app.listenForMail()
	go app.listenShutdown()
	go app.ListenForErrors()
	app.InfoLog.Println("Starting web srver....")

	err := srv.ListenAndServe()

	if err != nil {
		log.Panic(err)
	}
}

func (app *Config) ListenForErrors() {
	for {
		select {
		case err := <-app.ErrorChan:
			app.ErrLog.Println(err)
		case <-app.ErrorChanDone:
			return
		}
	}
}

func initSession() *scs.SessionManager {
	gob.Register(data.User{})
	session := scs.New()
	session.Store = redisstore.New(initRedis())
	session.Lifetime = 24 * time.Hour
	session.Cookie.Persist = true
	session.Cookie.SameSite = http.SameSiteLaxMode
	session.Cookie.Secure = true

	return session

}

func initRedis() *redis.Pool {
	redisPool := &redis.Pool{
		MaxIdle: 10,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", os.Getenv("REDIS"))

		},
	}
	return redisPool
}
func initDB() *sql.DB {
	conn := connectToDB()
	if conn == nil {
		log.Panic("cant connect to database")
	}
	return conn
}

func connectToDB() *sql.DB {
	counts := 0
	dsn := os.Getenv("DSN")

	for {
		connection, err := OpenDB(dsn)
		if err != nil {
			log.Println("postgress not ready yet", err)
		} else {
			log.Println("connected to database")
			return connection
		}

		if counts > 10 {
			return nil
		}

		log.Println("Backing off for 1 sec")
		time.Sleep(1 * time.Second)
		counts++
		continue
	}
}

func OpenDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (app *Config) listenShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	app.shutdown()
	os.Exit(0)
}

func (app *Config) createMail() Mail {
	errorChan := make(chan error)
	mailerChan := make(chan Message, 100)
	mailerDone := make(chan bool)
	m := Mail{
		Domain:      "localhost",
		Host:        "localhost",
		Port:        1025,
		Encryption:  "none",
		FromName:    "Info",
		FromAddress: "info@mycompany.com",
		Wait:        app.wait,
		ErrorChan:   errorChan,
		MailerChan:  mailerChan,
		DoneChan:    mailerDone,
	}

	return m
}

func (app *Config) shutdown() {
	app.InfoLog.Println("would run cleanup tasks...")

	app.wait.Wait()
	app.Mailer.DoneChan <- true
	app.ErrorChanDone <- true
	app.InfoLog.Println("closing channels and shutting down application")
	close(app.Mailer.DoneChan)
	close(app.Mailer.MailerChan)
	close(app.Mailer.ErrorChan)
	close(app.ErrorChan)
	close(app.ErrorChanDone)

}
