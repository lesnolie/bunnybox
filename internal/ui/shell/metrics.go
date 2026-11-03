package shell

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// historySamples is the traffic chart window: one sample per second.
const historySamples = 60

// Close cancels the read-only subscriptions before the backend closes.
func (s *Shell) Close() {
	s.metrics.stop()
	s.groupStream.stop()
	s.logStream.stop()
	s.connStream.stop()
}

// setService applies a polled service state. Healthy subscriptions survive the
// refresh: restarting the metrics stream would discard the chart and measured
// rate, because a new stream's first frame has none. Failed ones are rebuilt.
func (s *Shell) setService(state ServiceState) {
	live := s.service.Metrics
	continuing := s.service.Running && state.Running
	s.service = state
	if !continuing {
		return
	}
	if s.metrics.failed {
		s.metrics.stop()
	} else if s.metrics.started && live != nil {
		s.service.Metrics = live
	}
	if s.groupStream.failed {
		s.groupStream.stop()
	}
	if s.logStream.failed {
		s.logStream.stop()
	}
	if s.connStream.failed {
		s.connStream.stop()
	}
}

// updateStreams runs every frame: metrics follow a running core; groups and
// logs are subscribed only while their page is visible, like the Apple client.
func (s *Shell) updateStreams(c *ui.Context) {
	cb := s.config.Callbacks
	running := s.service.Running
	s.followService()
	if !running {
		s.metrics.stop()
		s.uploadHistory, s.downloadHistory = nil, nil
		s.groups, s.groupsReceived = nil, false
	}
	s.metrics.drain(s.applyMetrics, func(err error) {
		s.service.Metrics = nil
		s.uploadHistory, s.downloadHistory = nil, nil
		s.errorText = err.Error()
	})
	if running && !s.metrics.started && cb.WatchMetrics != nil {
		s.metrics.start(s.invalidator(c), cb.WatchMetrics)
	}
	wantGroups := running && s.page == "groups" && cb.WatchGroups != nil
	if !wantGroups {
		s.groupStream.stop()
	}
	s.groupStream.drain(func(groups []Group) {
		s.groups, s.groupsReceived = groups, true
	}, func(err error) {
		s.errorText = err.Error()
	})
	if wantGroups && !s.groupStream.started {
		s.groupStream.start(s.invalidator(c), cb.WatchGroups)
	}
	// Lines stay visible after the core stops; a new subscription replays
	// the service's saved lines with a reset, as Apple's reconnect does.
	wantLogs := running && s.page == "logs" && cb.WatchLogs != nil
	if !wantLogs {
		s.logStream.stop()
	}
	s.logStream.drain(s.logs.apply, func(err error) {
		s.errorText = err.Error()
	})
	if wantLogs && !s.logStream.started {
		s.logStream.start(s.invalidator(c), cb.WatchLogs)
	}
	// Connections belong to a running core; a new subscription starts with
	// a reset, so the table survives leaving the page.
	if !running {
		s.conns.table = connectionTable{}
		s.conns.detail, s.conns.detailOpen = nil, false
	}
	wantConns := running && s.page == "connections" && cb.WatchConnections != nil
	if !wantConns {
		s.connStream.stop()
	}
	s.connStream.drain(func(e ConnectionEvents) {
		s.conns.table.apply(e, time.Now().UnixMilli())
	}, func(err error) {
		s.errorText = err.Error()
	})
	if wantConns && !s.connStream.started {
		s.connStream.start(s.invalidator(c), cb.WatchConnections)
	}
	s.updateTools()
}

func (s *Shell) applyMetrics(m Metrics) {
	s.service.Metrics = &m
	if m.RateAvailable {
		s.uploadHistory = appendSample(s.uploadHistory, float32(m.UploadBytes))
		s.downloadHistory = appendSample(s.downloadHistory, float32(m.DownloadBytes))
	} else {
		s.uploadHistory, s.downloadHistory = nil, nil
	}
}

func appendSample(history []float32, sample float32) []float32 {
	if len(history) >= historySamples {
		history = append(history[:0], history[len(history)-historySamples+1:]...)
	}
	return append(history, max(0, sample))
}
