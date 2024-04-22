package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

const clientID = "Topic_counter" // Change this to something random if using a public test server
var topic = "zigbee2mqtt_g/+"
var topic2 = "zigbee2mqtt_g/+/set"

var mutex sync.Mutex = sync.Mutex{}

var store_from_zn = make(map[string]uint32, 100)
var store_to_zn = make(map[string]uint32, 100)

var store_from_zn_total = make(map[string]uint32, 100)
var store_to_zn_total = make(map[string]uint32, 100)

var resetEveryMinutes time.Duration = 60
var printEverySeconds time.Duration = 1
var username string = ""
var password string = ""
var mqttUrl string = "mqtt://xeon.lan:1883"

func checkCmdArgs() bool {
	cmdArgs := os.Args[1:]
	length := len(cmdArgs)

	for cmdOffset := 0; cmdOffset < length; cmdOffset++ {
		cmdArg := cmdArgs[cmdOffset]

		switch cmdArg {

		case "--help":
			{
				fmt.Println(" ==== HELP =====")
				fmt.Println("--reset Minutes to Reset all counts")
				fmt.Println("--print Seconds between Stats Printout")
				fmt.Println("--url MQTT Host URL (no username password)")
				fmt.Println("--user MQTT User")
				fmt.Println("--passwd MQTT Password")
				fmt.Println("--topic Topic")
				fmt.Println("--topic2 Topic")
				fmt.Println(" ==== END =====")
				return false
			}
		case "--reset":
			{
				reset, err := strconv.Atoi(cmdArgs[cmdOffset+1])
				if err != nil {
					fmt.Printf("Can't convert %s to an int!", cmdArgs[cmdOffset+1])
					cmdOffset++
					continue
				}
				resetEveryMinutes = time.Duration(reset)
				cmdOffset++
			}
		case "--print":
			{
				reset, err := strconv.Atoi(cmdArgs[cmdOffset+1])
				if err != nil {
					fmt.Printf("Can't convert %s to an int!", cmdArgs[cmdOffset+1])
					cmdOffset++
					continue
				}
				printEverySeconds = time.Duration(reset)
				cmdOffset++
			}
		case "--url":
			mqttUrl = cmdArgs[cmdOffset+1]
			cmdOffset++
		case "--user":
			username = cmdArgs[cmdOffset+1]
			cmdOffset++
		case "--passwd":
			password = cmdArgs[cmdOffset+1]
			cmdOffset++
		case "--topic":
			topic = cmdArgs[cmdOffset+1]
			cmdOffset++
		case "--topic2":
			topic2 = cmdArgs[cmdOffset+1]
			cmdOffset++
		}

	}
	return true
}

func main() {
	if !checkCmdArgs() {
		os.Exit(1)
	}
	// App will run until cancelled by user (e.g. ctrl-c)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// We will connect to the Eclipse test server (note that you may see messages that other users publish)
	u, err := url.Parse(mqttUrl)
	if err != nil {
		panic(err)
	}

	cliCfg := autopaho.ClientConfig{
		ConnectUsername: username,
		ConnectPassword: []byte(password),
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
			fmt.Println("mqtt connection up")
			// Subscribing in the OnConnectionUp callback is recommended (ensures the subscription is reestablished if
			// the connection drops)
			if _, err := cm.Subscribe(context.Background(), &paho.Subscribe{
				Subscriptions: []paho.SubscribeOptions{
					{Topic: topic, QoS: 0}, {Topic: topic2, QoS: 0},
				},
			}); err != nil {
				fmt.Printf("failed to subscribe (%s). This is likely to mean no messages will be received.", err)
			}
			fmt.Println("mqtt subscription made")
		},
		OnConnectError: func(err error) { fmt.Printf("error whilst attempting connection: %s\n", err) },
		// eclipse/paho.golang/paho provides base mqtt functionality, the below config will be passed in for each connection
		ClientConfig: paho.ClientConfig{
			// If you are using QOS 1/2, then it's important to specify a client id (which must be unique)
			ClientID: clientID,
			// OnPublishReceived is a slice of functions that will be called when a message is received.
			// You can write the function(s) yourself or use the supplied Router
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				func(pr paho.PublishReceived) (bool, error) {
					mutex.Lock()
					defer mutex.Unlock()

					rTopic := pr.Packet.Topic

					if strings.Contains(rTopic, "/set") {
						val, ok := store_to_zn[rTopic]
						if ok {
							store_to_zn[rTopic] = val + 1
						} else {
							store_to_zn[rTopic] = 1
						}
						val, ok = store_to_zn_total[rTopic]
						if ok {
							store_to_zn_total[rTopic] = val + 1
						} else {
							store_to_zn_total[rTopic] = 1
						}
					} else {
						val, ok := store_from_zn[rTopic]
						if ok {
							store_from_zn[rTopic] = val + 1
						} else {
							store_from_zn[rTopic] = 1
						}
						val, ok = store_from_zn_total[rTopic]
						if ok {
							store_from_zn_total[rTopic] = val + 1
						} else {
							store_from_zn_total[rTopic] = 1
						}
					}
					return true, nil
				}},
			OnClientError: func(err error) { fmt.Printf("client error: %s\n", err) },
			OnServerDisconnect: func(d *paho.Disconnect) {
				if d.Properties != nil {
					fmt.Printf("server requested disconnect: %s\n", d.Properties.ReasonString)
				} else {
					fmt.Printf("server requested disconnect; reason code: %d\n", d.ReasonCode)
				}
			},
		},
	}

	c, err := autopaho.NewConnection(ctx, cliCfg) // starts process; will reconnect until context cancelled
	if err != nil {
		panic(err)
	}
	// Wait for the connection to come up
	if err = c.AwaitConnection(ctx); err != nil {
		panic(err)
	}

	go printStats()
	go resetStats()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1)
	done := make(chan bool, 1)
	go func() {
		sig := <-sigs
		fmt.Println()
		fmt.Println(sig)
		done <- true
	}()
	fmt.Println("awaiting signal")
	<-done
	fmt.Println("exiting")

	fmt.Println("signal caught - exiting")
	<-c.Done() // Wait for clean shutdown (cancelling the context triggered the shutdown)

	fmt.Println("Writing alltime stats...")
	writeToFile_From(true)
	writeToFile_To(true)
}

func resetStats() {
	for range time.Tick(time.Minute * resetEveryMinutes) {
		fmt.Println("")
		fmt.Println("")
		doResetStats()
		fmt.Println("")
		fmt.Println("")
	}
}

func doResetStats() {
	mutex.Lock()
	defer mutex.Unlock()
	store_from_zn = make(map[string]uint32, 100)
	store_to_zn = make(map[string]uint32, 100)
}

func printStats() {
	for range time.Tick(time.Second * printEverySeconds) {
		fmt.Println("")
		fmt.Println("")
		printToZnStats()
		printFromZnStats()
		fmt.Println("")
		fmt.Println("")
		writeToFile_From(false)
		writeToFile_To(false)
	}
}

func printFromZnStats() {
	mutex.Lock()
	defer mutex.Unlock()

	fmt.Println("========= BEGINN FROM ZNET ========")
	keys := make([]string, 0, len(store_from_zn))

	for key := range store_from_zn {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return store_from_zn[keys[i]] < store_from_zn[keys[j]]
	})

	for _, k := range keys {
		fmt.Printf("%3d: %s\n", store_from_zn[k], k)
	}
	fmt.Println("========= END ========")
}

func printToZnStats() {
	mutex.Lock()
	defer mutex.Unlock()

	fmt.Println("========= BEGINN TO ZNET ========")
	keys := make([]string, 0, len(store_to_zn))

	for key := range store_to_zn {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return store_to_zn[keys[i]] < store_to_zn[keys[j]]
	})

	for _, k := range keys {
		fmt.Printf("%3d: %s\n", store_to_zn[k], k)
	}
	fmt.Println("========= END ========")
}

func writeToFile_From(total bool) {
	formatted := time.Now().Format(time.RFC3339)

	cwd, err_cwd := os.Getwd()
	if err_cwd != nil {
		cwd = "."
		fmt.Println(err_cwd)
	}

	var filePath = ""
	if total {
		filePath = fmt.Sprintf("%s/from_total_%s.json", cwd, formatted)
	} else {
		filePath = fmt.Sprintf("%s/from_%s.json", cwd, formatted)
	}
	f, err := os.Create(filePath)
	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()

	mutex.Lock()
	defer mutex.Unlock()

	b, err_marshal := json.Marshal(store_from_zn)
	if err_marshal != nil {
		fmt.Println(err_marshal)
	}

	_, err = f.Write(b)
	if err != nil {
		fmt.Println(err)
	}
}

func writeToFile_To(total bool) {
	formatted := time.Now().Format(time.RFC3339)

	cwd, err_cwd := os.Getwd()
	if err_cwd != nil {
		cwd = "."
		fmt.Println(err_cwd)
	}

	var filePath = ""
	if total {
		filePath = fmt.Sprintf("%s/to_total_%s.json", cwd, formatted)
	} else {
		filePath = fmt.Sprintf("%s/to_%s.json", cwd, formatted)
	}
	f, err := os.Create(filePath)
	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()

	mutex.Lock()
	defer mutex.Unlock()

	b, err_marshal := json.Marshal(store_to_zn)
	if err_marshal != nil {
		fmt.Println(err_marshal)
	}

	_, err = f.Write(b)
	if err != nil {
		fmt.Println(err)
	}
}
