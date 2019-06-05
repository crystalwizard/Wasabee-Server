package main

import (
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/cloudkucooland/WASABI"
	"github.com/cloudkucooland/WASABI/GroupMe"
	"github.com/cloudkucooland/WASABI/RISC"
	"github.com/cloudkucooland/WASABI/Telegram"
	"github.com/cloudkucooland/WASABI/http"
	"github.com/op/go-logging"
	"github.com/urfave/cli"
	// "runtime/pprof"
)

var flags = []cli.Flag{
	cli.StringFlag{
		Name: "database, d", EnvVar: "DATABASE", Value: "wasabi:GoodPassword@tcp(localhost)/wasabi",
		Usage: "MySQL/MariaDB connection string. It is recommended to pass this parameter as an environment variable."},
	cli.StringFlag{
		Name: "certs", EnvVar: "CERTDIR", Value: "./certs/",
		Usage: "Directory where HTTPS certificates are stored."},
	cli.StringFlag{
		Name: "root, r", EnvVar: "ROOT_URL", Value: "https://wasabi.phtiv.com",
		Usage: "The path under which the application will be reachable from the internet."},
	cli.StringFlag{
		Name: "wordlist", EnvVar: "WORD_LIST", Value: "eff_large_wordlist.txt",
		Usage: "Word list used for random slug generation."},
	cli.StringFlag{
		Name: "log", EnvVar: "LOGFILE", Value: "logs/wasabi.log",
		Usage: "output log file."},
	cli.StringFlag{
		Name: "https", EnvVar: "HTTPS_LISTEN", Value: ":443",
		Usage: "HTTPS listen address."},
	cli.StringFlag{
		Name: "httpslog", EnvVar: "HTTPS_LOGFILE", Value: "logs/wasabi-https.log",
		Usage: "HTTPS log file."},
	cli.StringFlag{
		Name: "frontend-path, p", EnvVar: "FRONTEND_PATH", Value: "./frontend",
		Usage: "Location of the frontend files."},
	cli.StringFlag{
		Name: "googleclient", EnvVar: "GOOGLE_CLIENT_ID", Value: "UNSET",
		Usage: "Google ClientID. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "googlesecret", EnvVar: "GOOGLE_CLIENT_SECRET", Value: "UNSET",
		Usage: "Google Client Secret. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "sessionkey", EnvVar: "SESSION_KEY", Value: "",
		Usage: "Session Key (32 char, random). It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "tgkey", EnvVar: "TELEGRAM_API_KEY", Value: "",
		Usage: "Telegram API Key. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "venlonekey", EnvVar: "VENLONE_API_KEY", Value: "",
		Usage: "V.enl.one API Key. It is recommended to pass this parameter as an environment variable"},
	cli.BoolFlag{
		Name: "venlonepoller", EnvVar: "VENLONE_POLLER",
		Usage: "Poll status.enl.one for RAID/JEAH location data."},
	cli.StringFlag{
		Name: "enlrockskey", EnvVar: "ENLROCKS_API_KEY", Value: "",
		Usage: "enl.rocks API Key. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "enliokey", EnvVar: "ENLIO_API_KEY", Value: "",
		Usage: "enl.io API token. It is recommended to pass this parameter as an environment variable"},
	cli.StringFlag{
		Name: "gmbotkey", EnvVar: "GROUPME_ACCESS_TOKEN", Value: "",
		Usage: "GroupMe Access Token."},
	cli.BoolFlag{
		Name: "debug", EnvVar: "DEBUG",
		Usage: "Show (a lot) more output."},
	cli.BoolFlag{
		Name:  "help, h",
		Usage: "Shows this help, then exits."},
}

func main() {
	app := cli.NewApp()

	app.Name = "WASABI"
	app.Version = "0.6.9"
	app.Usage = "WASABI Server"
	app.Authors = []cli.Author{
		{
			Name:  "Scot C. Bontrager",
			Email: "scot@indievisible.org",
		},
	}
	app.Copyright = "© Scot C. Bontrager"
	app.HelpName = "wasabi"
	app.Flags = flags
	app.HideHelp = true
	cli.AppHelpTemplate = strings.Replace(cli.AppHelpTemplate, "GLOBAL OPTIONS:", "OPTIONS:", 1)

	app.Action = run

	// f, _ := os.Create("logs/profile")
	// pprof.StartCPUProfile(f)
	// defer pprof.StopCPUProfile()

	_ = app.Run(os.Args)
}

func run(c *cli.Context) error {
	if c.Bool("help") {
		_ = cli.ShowAppHelp(c)
		return nil
	}

	if c.Bool("debug") {
		wasabi.SetLogLevel(logging.DEBUG)
	}
	if c.String("log") != "" {
		_ = wasabi.AddFileLog(c.String("log"), logging.INFO)
	}

	// Load words
	err := wasabi.LoadWordsFile(c.String("wordlist"))
	if err != nil {
		wasabi.Log.Errorf("Error loading word list from '%s': %s", c.String("wordlist"), err)
	}

	// load the UI templates
	ts, err := wasabi.TemplateConfig(c.String("frontend-path"))
	if err != nil {
		wasabi.Log.Errorf("unable to load frontend templates from %s; shutting down", c.String("frontend-path"))
		panic(err)
	}

	// Connect to database
	err = wasabi.Connect(c.String("database"))
	if err != nil {
		wasabi.Log.Errorf("Error connecting to database: %s", err)
		panic(err)
	}

	// setup V
	if c.String("venlonekey") != "" {
		wasabi.SetVEnlOne(c.String("venlonekey"))
		if c.Bool("venlonepoller") {
			go wasabi.StatusServerPoller()
		}
	}

	// setup Rocks
	if c.String("enlrockskey") != "" {
		wasabi.SetEnlRocks(c.String("enlrockskey"))
	}

	// setup enl.io
	if c.String("enliokey") != "" {
		wasabi.SetENLIO(c.String("enliokey"))
	}

	// Serve HTTPS
	if c.String("https") != "none" {
		go wasabihttps.StartHTTP(wasabihttps.Configuration{
			ListenHTTPS:      c.String("https"),
			FrontendPath:     c.String("frontend-path"),
			Root:             c.String("root"),
			CertDir:          c.String("certs"),
			GoogleClientID:   c.String("googleclient"),
			GoogleSecret:     c.String("googlesecret"),
			CookieSessionKey: c.String("sessionkey"),
			Logfile:          c.String("httpslog"),
			TemplateSet:      ts,
		})
	}

	riscPath := path.Join(c.String("certs"), "risc.json")
	if _, err := os.Stat(riscPath); err != nil {
		wasabi.Log.Noticef("%s does not exist, not enabling RISC", riscPath)
	} else {
		go risc.RISC(riscPath)
	}

	// Serve Telegram
	if c.String("tgkey") != "" {
		go wasabitelegram.WASABIBot(wasabitelegram.TGConfiguration{
			APIKey:      c.String("tgkey"),
			HookPath:    "/tg",
			TemplateSet: ts,
		})
	}

	// Serve Groupme
	if c.String("gmbotkey") != "" {
		go wasabigm.GMbot(wasabigm.GMConfiguration{
			AccessToken: c.String("gmbotkey"),
			TemplateSet: ts,
		})
	}

	// wait for signal to shut down
	sigch := make(chan os.Signal, 3)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)

	// loop until signal sent
	sig := <-sigch

	wasabi.Log.Info("Shutdown Requested: ", sig)
	if _, err := os.Stat(riscPath); err == nil {
		risc.DisableWebhook()
	}
	if r, _ := wasabi.TGRunning(); r {
		wasabitelegram.Shutdown()
	}
	if c.String("https") != "none" {
		_ = wasabihttps.Shutdown()
	}

	// close database connection
	wasabi.Disconnect()
	return nil
}
