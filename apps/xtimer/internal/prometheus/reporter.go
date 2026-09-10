package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// monitorComponentType is the type of a monitored metric.
type monitorComponentType string

const (
	// counter metric type.
	counter monitorComponentType = "counter"
	// summary metric type.
	summary monitorComponentType = "summary"
	// gauge metric type.
	gauge monitorComponentType = "gauge"

	// total number of timer trigger records.
	timerExecTotalCnt        = "timer_exec_total_cnt"
	timerExecTotalCntSummary = "total count of triggered timers"

	// timer trigger delay.
	timerDelayCnt        = "timer_delay_cnt"
	timerDelayCntSummary = "timer trigger delay"

	// total number of enabled timers.
	timerEnabledCnt        = "timer_enabled_cnt"
	timerEnabledCntSummary = "total count of enabled timers"

	// number of timers that never fired.
	timerUnexecutedCnt        = "timer_unexeced_cnt"
	timerUnexecutedCntSummary = "number of timers that did not fire on schedule"

	reportName = "_name"
	reportType = "_type"
	timerApp   = "xtimerApp"

	// common labels.
	label = "label"
	timer = "timer"
)

// Reporter is the metrics reporting service.
type Reporter struct {
	timerExecRecorder       *prometheus.CounterVec
	timeDelayRecorder       prometheus.ObserverVec
	timerEnabledRecorder    *prometheus.GaugeVec
	timerUnexecutedRecorder *prometheus.GaugeVec
}

var reporter = newReporter()

// GetReporter returns the singleton reporter.
func GetReporter() *Reporter {
	return reporter
}

// newReporter builds the metrics reporting service.
func newReporter() *Reporter {
	r := Reporter{
		// timer trigger records.
		timerExecRecorder: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: timerExecTotalCnt,
			Help: timerExecTotalCntSummary,
		}, []string{
			timerApp,
			reportName,
			reportType,
		}).MustCurryWith(prometheus.Labels{reportName: timerExecTotalCntSummary,
			reportType: string(counter)}),

		// timer delay records.
		timeDelayRecorder: promauto.NewSummaryVec(prometheus.SummaryOpts{
			Name:       timerDelayCnt,
			Help:       timerDelayCntSummary,
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001, 0.999: 0.0001, 0.9999: 0.00001},
		}, []string{
			timerApp,
			reportName,
			reportType,
		}).MustCurryWith(prometheus.Labels{reportName: timerDelayCntSummary,
			reportType: string(summary)}),

		// total number of enabled timers.
		timerEnabledRecorder: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: timerEnabledCnt,
			Help: timerEnabledCntSummary,
		}, []string{
			label,
			reportName,
			reportType,
		}).MustCurryWith(prometheus.Labels{reportName: timerEnabledCntSummary,
			reportType: string(gauge)}),

		// number of timers that never fired.
		timerUnexecutedRecorder: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: timerUnexecutedCnt,
			Help: timerUnexecutedCntSummary,
		}, []string{
			label,
			reportName,
			reportType,
		}).MustCurryWith(prometheus.Labels{reportName: timerUnexecutedCntSummary,
			reportType: string(gauge)}),
	}

	return &r
}

func (r *Reporter) ReportExecRecord(app string) {
	r.timerExecRecorder.WithLabelValues(app).Inc()
}

func (r *Reporter) ReportTimerDelayRecord(app string, cost float64) {
	r.timeDelayRecorder.WithLabelValues(app).Observe(cost)
}

func (r *Reporter) ReportTimerEnabledRecord(total float64) {
	r.timerEnabledRecorder.WithLabelValues(timer).Set(total)
}

func (r *Reporter) ReportTimerUnexecutedRecord(total float64) {
	r.timerUnexecutedRecorder.WithLabelValues(timer).Set(total)
}
