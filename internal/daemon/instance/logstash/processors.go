package logstash

import (
	"bufio"
	"io"
	"strings"

	"github.com/hpcloud/tail"
	"github.com/tidwall/gjson"
)

func (i *instance) stdoutProcessor(stdout io.ReadCloser) {
	defer i.logstashShutdownWG.Done()

	// The stdoutProcessor can only be started after the process is created.
	select {
	case <-i.logstashStarted:
	case <-i.ctxShutdown.Done():
		return
	}

	i.log.Debug("start stdout scanner")

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		i.log.Debugf("stdout:  %s", scanner.Text())

		// Only events starting with "{" are accepted. All other output is discarded.
		if !strings.HasPrefix(scanner.Text(), "{") {
			continue
		}

		i.controller.ReceiveEvent(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		i.log.Error("reading standard output:", err)
	}

	// Termination of stdout scanner is only expected, if shutdown is in progress.
	select {
	case <-i.ctxShutdown.Done():
	default:
		i.log.Warning("stdout scanner closed unexpectedly")
		i.controller.SignalCrash()
	}

	i.log.Debug("exit stdout scanner")
}

func (i *instance) stderrProcessor(stderr io.ReadCloser) {
	defer i.logstashShutdownWG.Done()

	// The stderrProcessor can only be started after the process is created.
	select {
	case <-i.logstashStarted:
	case <-i.ctxShutdown.Done():
		return
	}

	i.log.Debug("start stderr scanner")

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		i.log.Debugf("stderr:  %s", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		i.log.Error("reading standard err:", err)
	}

	// Termination of stderr scanner is only expected, if shutdown is in progress.
	select {
	case <-i.ctxShutdown.Done():
	default:
		i.log.Warning("stderr scanner closed unexpectedly")
		i.controller.SignalCrash()
	}

	i.log.Debug("exit stderr scanner")
}

func (i *instance) logstashLogProcessor(t *tail.Tail) {
	defer i.logstashShutdownWG.Done()

	for {
		select {
		case line := <-t.Lines:
			switch gjson.Get(line.Text, "logEvent.message").String() {
			case "Pipeline started":
				pipelineID := gjson.Get(line.Text, `logEvent.pipeline\.id`).String()
				i.log.Debugf("taillog: -> pipeline started: %s", pipelineID)

				i.controller.PipelinesReady(pipelineID)
			case "Pipeline started successfully": // Logstash < 7.0.0
				pipelineID := gjson.Get(line.Text, `logEvent.pipeline_id`).String()
				i.log.Debugf("taillog: -> pipeline started: %s", pipelineID)

				i.controller.PipelinesReady(pipelineID)
			case "Pipelines running":
				rp := gjson.Get(line.Text, `logEvent.running_pipelines.0.metaClass.metaClass.metaClass.running_pipelines`).String()
				runningPipelines := extractPipelines(rp)
				if runningPipelines == nil {
					// Since Logstash 8.11.x, the running pipelines are directly in the logEvent.
					rps := gjson.Get(line.Text, `logEvent.running_pipelines`).Array()
					for _, rp := range rps {
						runningPipelines = append(runningPipelines, rp.String())
					}
				}
				i.log.Debugf("taillog: -> pipeline running: %v", runningPipelines)

				i.controller.PipelinesReady(runningPipelines...)
				i.controller.PipelinesReady("__lfv_pipelines_running")
			}
		case <-i.ctxShutdown.Done():
			i.log.Debug("shutdown log reader")
			return
		}
	}
}

func extractPipelines(in string) []string {
	if len(in) < 3 {
		return nil
	}
	pipelines := strings.Split(in[1:len(in)-1], ",")
	for i := range pipelines {
		pipelines[i] = strings.TrimLeft(pipelines[i], " :")
		pipelines[i] = strings.Trim(pipelines[i], `"`)
	}
	return pipelines
}
