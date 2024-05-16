package main

import (
	"io"
	"maps"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
)

type ChartTimeData struct {
	content map[string]uint32
}

type ChartDataHolder struct {
	timedData map[time.Time]ChartTimeData
	topics    UniqueStringArray
}

func (d *ChartDataHolder) ChartPushData(data map[string]uint32) {
	d.timedData[time.Now()] = ChartTimeData{content: maps.Clone(data)}
	topics, _ := MapKeys(data)
	d.topics.AddStrings(topics...)
}

func (d *ChartDataHolder) toLineItems(topic string, times *[]time.Time) []opts.LineData {
	ld := make([]opts.LineData, 0)
	for _, t := range *times {
		val, found := d.timedData[t].content[topic]
		if found {
			ldd := opts.LineData{Value: val}
			ld = append(ld, ldd)
		}
	}
	return ld
}

func (d *ChartDataHolder) GenChart(writer io.Writer, title string, subtitle string) error {
	page := components.NewPage()
	page.Layout = components.PageFlexLayout
	page.Width = "100%"
	page.Height = "100%"

	times, _ := MapKeys(d.timedData)
	topics, _ := d.topics.getStrings()

	line := charts.NewLine()

	toolbox := opts.Toolbox{Show: true}
	toolbox.Feature = new(opts.ToolBoxFeature)
	toolbox.Feature.SaveAsImage = new(opts.ToolBoxFeatureSaveAsImage)
	toolbox.Feature.SaveAsImage.Show = true
	toolbox.Feature.DataZoom = new(opts.ToolBoxFeatureDataZoom)
	toolbox.Feature.DataZoom.Show = true
	toolbox.Feature.DataView = new(opts.ToolBoxFeatureDataView)
	toolbox.Feature.DataView.Show = true
	toolbox.Feature.Restore = new(opts.ToolBoxFeatureRestore)
	toolbox.Feature.Restore.Show = true

	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: "dark", PageTitle: "mqtt_topics", Width: "100%", Height: "100vh"}),
		charts.WithTitleOpts(opts.Title{
			Title:    title,
			Subtitle: subtitle,
		}),
		charts.WithLegendOpts(opts.Legend{Type: "scroll"}),
		charts.WithTooltipOpts(opts.Tooltip{Show: true, Trigger: "axis", TriggerOn: "mousemove"}),
		charts.WithAnimation(),
		charts.WithDataZoomOpts(opts.DataZoom{Type: "inside"}),
		charts.WithToolboxOpts(toolbox),
	)

	line.SetXAxis(times) //.
	//AddSeries("Category A", generateLineItems()).
	//AddSeries("Category B", generateLineItems()).

	for _, topic := range topics {
		line.AddSeries(topic, d.toLineItems(topic, &times))
	}

	line.SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	//page.AddCharts(line)
	//return page.Render(writer)
	return line.Render(writer)
}
