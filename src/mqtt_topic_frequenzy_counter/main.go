package main

import (
	"context"
	"errors"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/go-co-op/gocron/v2"
)

var topicProcs []*TopicProc

var (
	WarningLogger *log.Logger
	InfoLogger    *log.Logger
	ErrorLogger   *log.Logger
)

func doSubscribe(ctx context.Context, cm *autopaho.ConnectionManager, prop *paho.ConnackProperties) {
	for _, entry := range topicProcs {

		subprop := new(paho.SubscribeProperties)
		subprop.SubscriptionIdentifier = new(int)
		*subprop.SubscriptionIdentifier = entry.subID
		subprop.User = prop.User

		subopt := []paho.SubscribeOptions{
			{
				Topic: entry.baseTopic,
				QoS:   0,
			},
		}

		subscribe := new(paho.Subscribe)
		subscribe.Subscriptions = subopt
		subscribe.Properties = subprop

		ack, err := cm.Subscribe(ctx, subscribe)
		if err == nil {
			InfoLogger.Printf("Subcribe sucess: %#v", ack)
		} else {
			ErrorLogger.Println(err)
		}
	}
}

func setupMqtt(args ParsedArgs, settings *SettingsStruct, ctx context.Context) (*autopaho.ConnectionManager, error) {
	var user string
	var passwd string
	var cID string
	var Url string
	var err error

	user, err = getBetterString(args.username, settings.User)
	if err != nil {
		WarningLogger.Println("No usable username! Trying anyway...")
	}

	passwd, err = getBetterString(args.password, settings.Passwd)
	if err != nil {
		WarningLogger.Println("No usable password! Trying anyway...")
	}

	Url, err = getBetterString(args.mqttUrl, settings.Url)
	if err != nil {
		ErrorLogger.Println("No usable url! Bye")
		return nil, errors.New("MQTT: No usable url")
	}

	cID, err = getBetterString(settings.ClientID, "Topic Analyzer")
	if err != nil {
		ErrorLogger.Println("No usable client_id! Bye")
		return nil, errors.New("MQTT: No usable client_id")
	}

	// We will connect to the Eclipse test server (note that you may see messages that other users publish)
	u, err := url.Parse(Url)
	if err != nil {
		return nil, err
	}

	cliCfg := autopaho.ClientConfig{
		//Debug:           InfoLogger,
		//PahoDebug:       InfoLogger,
		Errors:          ErrorLogger,
		PahoErrors:      ErrorLogger,
		ConnectUsername: user,
		ConnectPassword: []byte(passwd),
		ServerUrls:      []*url.URL{u},
		KeepAlive:       20, // Keepalive message should be sent every 20 seconds
		// CleanStartOnInitialConnection defaults to false. Setting this to true will clear the session on the first connection.
		CleanStartOnInitialConnection: false,
		// SessionExpiryInterval - Seconds that a session will survive after disconnection.
		// It is important to set this because otherwise, any queued messages will be lost if the connection drops and
		// the server will not queue messages while it is down. The specific setting will depend upon your needs
		// (60 = 1 minute, 3600 = 1 hour, 86400 = one day, 0xFFFFFFFE = 136 years, 0xFFFFFFFF = don't expire)
		SessionExpiryInterval: 3600,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
			InfoLogger.Println("mqtt connection up")
			// Subscribing in the OnConnectionUp callback is recommended (ensures the subscription is reestablished if
			// the connection drops)
			doSubscribe(ctx, cm, connAck.Properties)
			InfoLogger.Println("mqtt subscription made")
		},
		OnConnectError: func(err error) { ErrorLogger.Printf("error whilst attempting connection: %s\n", err) },
		// eclipse/paho.golang/paho provides base mqtt functionality, the below config will be passed in for each connection
		ClientConfig: paho.ClientConfig{
			// If you are using QOS 1/2, then it's important to specify a client id (which must be unique)
			ClientID: cID,
			// OnPublishReceived is a slice of functions that will be called when a message is received.
			// You can write the function(s) yourself or use the supplied Router
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) {
					found := false

					for _, val := range topicProcs {
						if val.subID == *pr.Packet.Properties.SubscriptionIdentifier {
							found = true
							val.process(pr.Packet.Topic)
						}
					}

					if !found {
						ErrorLogger.Printf("%s with subid: %d was not found in my list OoO", pr.Packet.Topic, *pr.Packet.Properties.SubscriptionIdentifier)
						return false, nil
					}

					return true, nil
				}},
			OnClientError: func(err error) { ErrorLogger.Printf("client error: %s\n", err) },
			OnServerDisconnect: func(d *paho.Disconnect) {
				if d.Properties != nil {
					InfoLogger.Printf("server requested disconnect: %s\n", d.Properties.ReasonString)
				} else {
					InfoLogger.Printf("server requested disconnect; reason code: %d\n", d.ReasonCode)
				}
			},
		},
	}

	c, err := autopaho.NewConnection(ctx, cliCfg) // starts process; will reconnect until context cancelled
	if err != nil {
		ErrorLogger.Panicln(err)
	}
	// Wait for the connection to come up
	if err = c.AwaitConnection(ctx); err != nil {
		ErrorLogger.Panicln(err)
	}

	return c, nil
}

func main() {

	InfoLogger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	WarningLogger = log.New(os.Stdout, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	scheduler, err := gocron.NewScheduler()
	if err != nil {
		ErrorLogger.Panicln(err)
	}

	scheduler.Start()

	var args ParsedArgs
	args, err = checkCmdArgs()
	if err != nil {
		WarningLogger.Println(err)
		return
	}

	settings, serr := loadSettings(args.settings, true, InfoLogger)
	if serr != nil {
		WarningLogger.Println(err)
	}

	fman := fileman{
		working_directory: getBetterStringNoErr(args.path, settings.Path),
	}

	for idx, entry := range settings.Topics {
		tp, err := NewTopicProc(entry, scheduler, InfoLogger)
		if err != nil {
			ErrorLogger.Printf("Setting up TopicProc for %s (%s) failed: %#v\n", entry.Topic, entry.FriendlyName, err)
			continue
		}

		tp.subID = idx + 1
		tp.fman = &fman
		topicProcs = append(topicProcs, tp)
	}

	// App will run until cancelled by user (e.g. ctrl-c)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var conn *autopaho.ConnectionManager
	conn, err = setupMqtt(args, settings, ctx)

	if err != nil {
		ErrorLogger.Panicln(err)
	}

	if args.graph != 0 {
		InfoLogger.Println("Graph gneration enabled, you can send SIGUSR1 to process to write graph and beginn new session")
		usr1 := make(chan os.Signal, 1)
		signal.Notify(usr1, syscall.SIGUSR1)
		go func() {
			for {
				su := <-usr1
				InfoLogger.Printf("[GOT: %s] OK, gen graph & reset...\n", su.String())
				for _, tp := range topicProcs {
					tp.writeToJsonFile(false)
					tp.writeStatsConsole()
					tp.writeGraph()
				}
				InfoLogger.Println("Done")
			}
		}()
	}

	if args.repr {
		InfoLogger.Println("repr found skipping go routine printStats")
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)
	go func() {
		sig := <-sigs
		InfoLogger.Println()
		InfoLogger.Println(sig)
		done <- true
	}()
	InfoLogger.Println("awaiting signal")
	<-done
	InfoLogger.Println("exiting")

	InfoLogger.Println("signal caught - exiting")
	<-conn.Done() // Wait for clean shutdown (cancelling the context triggered the shutdown)

	scheduler.Shutdown()

	InfoLogger.Println("Writing alltime stats...")
	for _, tp := range topicProcs {
		tp.writeToJsonFile(true)
	}

	InfoLogger.Println("Writing charts...")
	for _, tp := range topicProcs {
		tp.writeGraph()
	}
	InfoLogger.Println("Bye!")
}
