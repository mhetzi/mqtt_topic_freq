package main

import (
	"errors"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	ETC_SETTINGS_PATH = "/etc/default/mqtt_freq_analyzer.yaml"
)

/*
friendly_name: Name in chart legend
topic: the topic to watch
save_chart: cron string
save_json: Cron string
*/
type SettingsTopicEntry struct {
	FriendlyName   string   `yaml:"friendly_name"`
	Topic          string   `yaml:"topic"`
	SaveChartCron  string   `yaml:"save_chart"`
	SaveStatsCron  string   `yaml:"save_json"`
	ResetStatsCron string   `yaml:"reset_data"`
	IgnoreTopics   []string `yaml:"exclude_topics"`
}

type SettingsStruct struct {
	Topics   []SettingsTopicEntry `yaml:"topics"`
	Url      string               `yaml:"url"`
	User     string               `yaml:"user"`
	Passwd   string               `yaml:"password"`
	ClientID string               `yaml:"client_id"`
}

func loadSettings(path string, debug bool, log *log.Logger) (*SettingsStruct, error) {
	lp := ETC_SETTINGS_PATH
	if len(path) > 0 {
		lp = path
	}

	newSettings := new(SettingsStruct)
	newSettings.Topics = make([]SettingsTopicEntry, 0)

	log.Printf("Checking for Settingsfile %s ...", lp)
	if _, err := os.Stat(lp); errors.Is(err, os.ErrNotExist) {
		return newSettings, err
	}

	log.Println("Reading Settingsfile...")
	file, err := os.ReadFile(lp)

	if err != nil {
		return newSettings, err
	}

	log.Println("Parsing Settings...")
	temp := newSettings
	err = yaml.Unmarshal(file, temp)
	if err != nil {
		return nil, err
	}

	if debug {
		log.Printf("yaml: %#v\n", temp)
	}

	log.Printf("yaml: is sane? %t\n", newSettings.sanitize())

	return newSettings, nil
}

func (d *SettingsStruct) sanitize() bool {
	if len(d.ClientID) == 0 {
		d.ClientID = "mqtt_topic_freq"
	}
	if len(d.Url) == 0 {
		return false
	}
	return true
}

type EmptyString struct{}

func (EmptyString) Error() string {
	return "Both strings are empty"
}

func getBetterString(s1 string, s2 string) (string, error) {
	if len(s1) > 0 {
		return s1, nil
	}
	if len(s2) > 0 {
		return s2, nil
	}
	return "", new(EmptyString)
}
