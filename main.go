package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"sort"
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

var chartTo *ChartDataHolder = nil
var chartFrom *ChartDataHolder = nil

func initChartData() {
	mutex.Lock()
	defer mutex.Unlock()

	chartTo = new(ChartDataHolder)
	chartTo.timedData = make(map[time.Time]ChartTimeData, 10)
	chartTo.topics.array = make(map[string]bool, 10)

	chartFrom = new(ChartDataHolder)
	chartFrom.timedData = make(map[time.Time]ChartTimeData, 10)
	chartFrom.topics.array = make(map[string]bool, 10)
}

func main() {
	args, err := checkCmdArgs()
	if err != nil {
		return
	}

	// App will run until cancelled by user (e.g. ctrl-c)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// We will connect to the Eclipse test server (note that you may see messages that other users publish)
	u, err := url.Parse(args.mqttUrl)
	if err != nil {
		panic(err)
	}

	cliCfg := autopaho.ClientConfig{
		ConnectUsername: args.username,
		ConnectPassword: []byte(args.password),
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

	if args.graph != 0 {
		initChartData()
		if args.graph > 0 {
			go writeGraphTimed(args.graph)
		}
		fmt.Println("Graph gneration enabled, you can send SIGUSR1 to process to write graph and beginn new session")
		usr1 := make(chan os.Signal, 1)
		signal.Notify(usr1, syscall.SIGUSR1)
		go func() {
			for {
				su := <-usr1
				fmt.Printf("[GOT: %s] OK, gen graph & reset...\n", su.String())
				writeGraph()
				initChartData()
				fmt.Println("Done")
			}
		}()
	}

	if args.repr {
		fmt.Println("repr found skipping go routine printStats")
	} else {
		go printStats(args.printEverySeconds)
	}
	go resetStats(args.resetEveryMinutes, args.repr)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
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

	fmt.Println("Writing charts...")
	doPushChart()
	writeGraph()
	fmt.Println("Bye!")
}

func resetStats(everyMinutes int, repr bool) {
	for range time.Tick(time.Minute * time.Duration(everyMinutes)) {
		doPushChart()
		fmt.Println("")
		fmt.Println("")
		if repr {
			fmt.Println("resetStats repr")
			printToZnStats()
			printFromZnStats()
		}
		doResetStats()
		fmt.Println("")
		fmt.Println("")
	}
}

func doPushChart() {
	mutex.Lock()
	defer mutex.Unlock()

	if chartFrom == nil || chartTo == nil {
		return
	}

	chartTo.ChartPushData(store_to_zn)
	chartFrom.ChartPushData(store_from_zn)
}

func doResetStats() {
	mutex.Lock()
	defer mutex.Unlock()
	store_from_zn = make(map[string]uint32, 100)
	store_to_zn = make(map[string]uint32, 100)
}

func printStats(every_Seconds int) {
	for range time.Tick(time.Second * time.Duration(every_Seconds)) {
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
	tot := ""
	if total {
		tot = "total"
	}

	f, err := getFileWithTimestamp("from", tot, "json")
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
	tot := ""
	if total {
		tot = "total"
	}

	f, err := getFileWithTimestamp("to", tot, "json")

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

func writeGraphTimed(everyMinutes int) {
	for range time.Tick(time.Minute * time.Duration(everyMinutes)) {
		fmt.Println("Writing charts...")
		writeGraph()
		initChartData()
	}
}

func writeGraph() {
	mutex.Lock()
	defer mutex.Unlock()

	if chartFrom != nil {
		ff, err := getFileWithTimestamp("graph", "from", "html")
		if err != nil {
			fmt.Println(err)
			return
		}
		defer ff.Close()

		chartFrom.GenChart(ff, "From Zigbee Network", time.Now().Format(time.RFC3339))
	}
	if chartTo != nil {
		ft, err := getFileWithTimestamp("graph", "to", "html")
		if err != nil {
			fmt.Println(err)
			return
		}
		defer ft.Close()

		chartTo.GenChart(ft, "To Zigbee Network", time.Now().Format(time.RFC3339))
	}
}
