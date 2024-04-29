package main

import (
	"encoding/json"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
)

type topicMap map[string]uint32

type TopicProc struct {
	_log            *log.Logger
	_mutex          sync.Mutex
	_job_chart      gocron.Job
	_job_json       gocron.Job
	_job_reset      gocron.Job
	_exc_topics     []string
	baseTopic       string
	friendlyName    string
	topicStore      topicMap
	topicStoreTotal topicMap
	chart           ChartDataHolder
	subID           int
}

func (d *TopicProc) process(topic string) bool {
	d._mutex.Lock()
	defer d._mutex.Unlock()

	for _, s := range d._exc_topics {
		if strings.Contains(topic, s) {
			return true
		}
	}

	val, ok := d.topicStore[topic]
	if ok {
		d.topicStore[topic] = val + 1
	} else {
		d.topicStore[topic] = 1
	}
	val, ok = d.topicStoreTotal[topic]
	if ok {
		d.topicStoreTotal[topic] = val + 1
	} else {
		d.topicStoreTotal[topic] = 1
	}

	return true
}

func (d *TopicProc) writeGraph() error {
	d._mutex.Lock()
	defer d._mutex.Unlock()

	ff, err := getFileWithTimestamp("graph", d.friendlyName, "html")
	if err != nil {
		return err
	}
	defer ff.Close()

	return d.chart.GenChart(ff, d.friendlyName, time.Now().Format(time.RFC3339))
}

/* Internal function, cuncurrent unsafe */
func (d *TopicProc) _getKeysSortedByValue() []string {
	keys, _ := MapKeys(d.topicStore)
	sort.SliceStable(keys, func(i, j int) bool {
		return d.topicStore[keys[i]] < d.topicStore[keys[j]]
	})
	return keys
}

func (d *TopicProc) writeStatsConsole() {
	d._mutex.Lock()
	defer d._mutex.Unlock()
	keys := d._getKeysSortedByValue()

	log.Printf("========= BEGINN %s ========\n", d.friendlyName)
	for _, k := range keys {
		log.Printf("%3d: %s\n", d.topicStore[k], k)
	}
	log.Println("========= END ========")
}

func (d *TopicProc) writeToJsonFile(total bool) error {
	tot := ""
	if total {
		tot = "total"
	}

	f, err := getFileWithTimestamp(d.friendlyName, tot, "json")
	if err != nil {
		return err
	}
	defer f.Close()

	d._mutex.Lock()
	defer d._mutex.Unlock()

	b, err_marshal := json.Marshal(d.topicStore)
	if err_marshal != nil {
		log.Println(err_marshal)
	}

	_, err = f.Write(b)
	if err != nil {
		return err
	}
	return nil
}

func (d *TopicProc) _InitTopicProc() {
	d._mutex = sync.Mutex{}
	d.topicStore = make(topicMap, 20)
	d.topicStoreTotal = make(topicMap, 20)
	d.chart = ChartDataHolder{
		timedData: make(map[time.Time]ChartTimeData),
		topics: UniqueStringArray{
			array: make(map[string]bool),
		},
	}
	d.friendlyName = ""
}

/* Important for Charting, pushes data to chart & resets */
func (d *TopicProc) ResetStats() {
	d._mutex.Lock()
	defer d._mutex.Unlock()

	d.chart.ChartPushData(d.topicStore)
	d.topicStore = make(topicMap, len(d.topicStore))

}

func NewTopicProc(setting SettingsTopicEntry, sched gocron.Scheduler, log *log.Logger) (*TopicProc, error) {
	name, err := getBetterString(setting.FriendlyName, setting.Topic)

	if err != nil {
		return nil, err
	}

	d := new(TopicProc)
	d._InitTopicProc()
	d.baseTopic = setting.Topic
	d.friendlyName = name
	d._exc_topics = setting.IgnoreTopics

	if len(setting.SaveChartCron) > 0 {
		d._job_chart, err = sched.NewJob(gocron.CronJob(setting.SaveChartCron, true), gocron.NewTask(
			func() {
				d.writeGraph()

				jcnr, jcnre := d._job_chart.NextRun()
				log.Printf("ChartCron: UUID: %s, NextRun: %+v, NextRunErr: %+v\n", d._job_chart.ID(), jcnr, jcnre)
			},
		))
		if err != nil {
			return nil, err
		}
		jcnr, jcnre := d._job_chart.NextRun()
		log.Printf("ChartCron: UUID: %s, NextRun: %+v, NextRunErr: %+v\n", d._job_chart.ID(), jcnr, jcnre)
	}

	if len(setting.SaveStatsCron) > 0 {
		d._job_json, err = sched.NewJob(gocron.CronJob(setting.SaveStatsCron, true), gocron.NewTask(
			func() {
				d.writeStatsConsole()
				d.writeToJsonFile(false)
				jjnr, jjnre := d._job_json.NextRun()
				log.Printf("StatsCron: UUID: %s, NextRun: %+v, NextRunErr: %+v\n", d._job_json.ID(), jjnr, jjnre)
			},
		))
		if err != nil {
			return nil, err
		}
		jjnr, jjnre := d._job_json.NextRun()
		log.Printf("StatsCron: UUID: %s, NextRun: %+v, NextRunErr: %+v\n", d._job_json.ID(), jjnr, jjnre)
	}

	if len(setting.ResetStatsCron) > 0 {
		d._job_reset, err = sched.NewJob(gocron.CronJob(setting.ResetStatsCron, true), gocron.NewTask(
			func() {
				if d._job_json == nil {
					log.Println("No stats cron defined, repr implied")
					d.writeStatsConsole()
				}
				d.ResetStats()
				jjnr, jjnre := d._job_reset.NextRun()
				log.Printf("ResetCron: UUID: %s, NextRun: %+v, NextRunErr: %+v\n", d._job_reset.ID(), jjnr, jjnre)
			},
		))
		if err != nil {
			return nil, err
		}
		jjnr, jjnre := d._job_reset.NextRun()
		log.Printf("ResetCron: UUID: %s, NextRun: %+v, NextRunErr: %+v\n", d._job_reset.ID(), jjnr, jjnre)
	}

	return d, nil
}
