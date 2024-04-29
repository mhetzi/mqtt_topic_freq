package main

import (
	"fmt"
	"os"
	"strconv"
)

type ArgsToExit struct {
	msg string
}

func (e ArgsToExit) Error() string {
	return "Requesting exit due to spicific arguments found"
}

type ParsedArgs struct {
	resetEveryMinutes int
	printEverySeconds int
	username          string
	password          string
	mqttUrl           string
	settings          string

	topic  string
	topic2 string
	graph  int
	repr   bool
}

func checkCmdArgs() (ParsedArgs, error) {
	retArgs := ParsedArgs{
		resetEveryMinutes: 60,
		printEverySeconds: 60,
		username:          "",
		password:          "",
		mqttUrl:           "",
		topic:             "",
		topic2:            "",
		settings:          "",
	}

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
				fmt.Println("--repr Do print and reset immidiatly")
				fmt.Println("--graph Generate graphs,  datapoints are collected on reset or an SIGUSR1")
				fmt.Println("--grm Render graph every x Minutes")
				fmt.Println("--config PATH Use custom config Path")
				fmt.Println(" ==== END =====")
				return retArgs, &ArgsToExit{msg: "help"}
			}
		case "--reset":
			{
				reset, err := strconv.Atoi(cmdArgs[cmdOffset+1])
				if err != nil {
					fmt.Printf("Can't convert %s to an int!", cmdArgs[cmdOffset+1])
					cmdOffset++
					continue
				}
				retArgs.resetEveryMinutes = reset
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
				retArgs.printEverySeconds = reset
				cmdOffset++
			}
		case "--url":
			retArgs.mqttUrl = cmdArgs[cmdOffset+1]
			cmdOffset++
		case "--user":
			retArgs.username = cmdArgs[cmdOffset+1]
			cmdOffset++
		case "--passwd":
			retArgs.password = cmdArgs[cmdOffset+1]
			cmdOffset++
		case "--topic":
			retArgs.topic = cmdArgs[cmdOffset+1]
			cmdOffset++
		case "--topic2":
			retArgs.topic2 = cmdArgs[cmdOffset+1]
			cmdOffset++
		case "--repr":
			retArgs.repr = true
		case "--graph":
			if retArgs.graph == 0 {
				retArgs.graph = -1
			}
		case "--grm":
			i, e := strconv.Atoi(cmdArgs[cmdOffset+1])
			if e == nil {
				retArgs.graph = i
				cmdOffset++
			} else {
				fmt.Println("--grm argument is invalid")
				fmt.Println(e)
			}
		case "--config":
			retArgs.settings = cmdArgs[cmdOffset+1]
		}

	}
	return retArgs, nil
}
