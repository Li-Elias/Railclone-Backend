package main

import (
	"flag"
	"os"
	"strings"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Li-Elias/Railclone/internal/db"
	"github.com/Li-Elias/Railclone/internal/jsonlog"
	"github.com/Li-Elias/Railclone/internal/mail"
	"github.com/Li-Elias/Railclone/internal/models"
)

type config struct {
	port int
	env  string
	cors struct {
		allowedOrigins []string
	}
	kubeconfig string
	db.DB
	mail.SMTP
}

type application struct {
	config    config
	logger    *jsonlog.Logger
	waitgroup sync.WaitGroup
	models    models.Models
	mailer    mail.Mailer
	clientset *kubernetes.Clientset
}

func main() {
	var cfg config

	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)

	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	flag.StringVar(&cfg.DB.Dsn, "db-dsn", "", "PostgreSQL DSN")
	flag.IntVar(&cfg.DB.MaxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.DB.MaxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(
		&cfg.DB.MaxIdleTime,
		"db-max-idle-time",
		"15m",
		"PostgreSQL max connection idle time",
	)

	flag.StringVar(&cfg.SMTP.Host, "smtp-host", "sandbox.smtp.mailtrap.io", "SMTP host")
	flag.IntVar(&cfg.SMTP.Port, "smtp-port", 25, "SMTP port")
	flag.StringVar(&cfg.SMTP.Username, "smtp-username", "", "SMTP username")
	flag.StringVar(&cfg.SMTP.Password, "smtp-password", "", "SMTP password")
	flag.StringVar(&cfg.SMTP.Sender, "smtp-sender", "<no-reply@file-transfer.io>", "SMTP sender")

	flag.StringVar(&cfg.kubeconfig, "kubeconfig", "", "absolute path to kubeconfig file")

	flag.Func(
		"cors-allowed-origins",
		"Allowed CORS origins (space separated)",
		func(val string) error {
			cfg.cors.allowedOrigins = strings.Fields(val)
			return nil
		},
	)

	flag.Parse()

	db, err := db.Init(&cfg.DB)
	if err != nil {
		logger.PrintFatal(err, nil)
	}
	defer db.Close()
	logger.PrintInfo("database connection pool established", nil)

	config, err := clientcmd.BuildConfigFromFlags("", cfg.kubeconfig)
	if err != nil {
		logger.PrintFatal(err, nil)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.PrintFatal(err, nil)
	}
	logger.PrintInfo("kubernetes clientset established", nil)

	app := &application{
		config:    cfg,
		logger:    logger,
		models:    models.NewModels(db),
		mailer:    mail.New(&cfg.SMTP),
		clientset: clientset,
	}

	err = app.serve()
	if err != nil {
		logger.PrintFatal(err, nil)
	}
}
