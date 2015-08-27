package main // import "github.com/christian-blades-cb/docker-event-metrics"

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/jessevdk/go-flags"
	"github.com/quipo/statsd"
	"strings"
	"time"
)

var opts struct {
	StatsDHost          string       `description:"where to send statsd metrics" short:"s" long:"statsd-host" default:"localhost:8125" env:"STATSD_HOST"`
	StatsDPrefix        string       `description:"prefix for metrics in statsd" long:"statsd-prefix" default:"docker." env:"STATSD_PREFIX"`
	StatsDPurgeInterval func(string) `description:"frequency (in seconds) to send metrics to the statsd collector" long:"statsd-purge-frequency" default:"15s" env:"STATSD_FREQUENCY"`
	statsdFrequency     time.Duration

	DockerEndpoint string `description:"where to find docker" long:"docker-endpoint" default:"unix:///var/run/docker.sock" env:"DOCKER_ENDPOINT"`
}

var stats *statsd.StatsdBuffer

func main() {
	opts.StatsDPurgeInterval = func(durString string) {
		if dur, err := time.ParseDuration(durString); err != nil {
			log.WithField("freetext_duration", durString).Fatal("could not parse purge frequency")
		} else {
			opts.statsdFrequency = dur
		}
	}
	if _, err := flags.Parse(&opts); err != nil {
		log.WithField("error", err).Fatal("unable to parse arguments")
	}

	stats = mustStartStatsd(opts.StatsDHost, opts.StatsDPrefix, opts.statsdFrequency)

	mustListenToEvents(opts.DockerEndpoint)

	log.Debug("hello, world")
}

func mustStartStatsd(host string, prefix string, frequency time.Duration) *statsd.StatsdBuffer {
	sClient := statsd.NewStatsdClient(opts.StatsDHost, opts.StatsDPrefix)
	if err := sClient.CreateSocket(); err != nil {
		log.WithField("error", err).Fatal("could not open socket to statsd")
	}

	return statsd.NewStatsdBuffer(opts.statsdFrequency, sClient)
}

func mustListenToEvents(endpoint string) {
	dockerEvents := make(chan *docker.APIEvents)

	client, err := docker.NewClient(opts.DockerEndpoint)
	if err != nil {
		log.WithField("error", err).Fatal("could not contact docker daemon")
	}

	if err = client.AddEventListener(dockerEvents); err != nil {
		log.WithField("error", err).Fatal("error attaching to docker event stream")
	}

	for event := range dockerEvents {
		metricEvent(stats, client, event)
	}
}

func metricEvent(s *statsd.StatsdBuffer, c *docker.Client, event *docker.APIEvents) {
	s.Incr(event.Status, 1)
	container, err := c.InspectContainer(event.ID)
	if err != nil {
		log.WithFields(log.Fields{"containerid": event.ID, "error": err}).Warn("could not inspect container")
		return
	}

	s.Incr(fmt.Sprintf("%s.%s", event.Status, shortenImageName(container.Config.Image)), 1)
}

func shortenImageName(img string) string {
	lowerBound := strings.LastIndex(img, "/") + 1
	upperBound := strings.Index(img, ":")
	if upperBound == -1 {
		upperBound = len(img)
	}

	return img[lowerBound:upperBound]
}
