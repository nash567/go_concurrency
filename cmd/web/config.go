package main

import (
	"database/sql"
	"log"
	"sync"

	"github.com/alexedwards/scs/v2"
	"github.com/messageService/data"
)

type Config struct {
	Session       *scs.SessionManager
	DB            *sql.DB
	InfoLog       *log.Logger
	ErrLog        *log.Logger
	wait          *sync.WaitGroup
	Models        data.Models
	Mailer        Mail
	ErrorChan     chan error
	ErrorChanDone chan bool
}
