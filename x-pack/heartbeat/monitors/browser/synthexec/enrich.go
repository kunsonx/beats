// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package synthexec

import (
	"fmt"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat/events"
	"github.com/elastic/beats/v7/libbeat/processors/add_data_stream"

	"github.com/gofrs/uuid"

	"github.com/elastic/beats/v7/heartbeat/eventext"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

type enricher func(event *beat.Event, se *SynthEvent, fields StdSuiteFields) error

type streamEnricher struct {
	je *journeyEnricher
}

func (e *streamEnricher) enrich(event *beat.Event, se *SynthEvent, fields StdSuiteFields) error {
	if e.je == nil || (se != nil && se.Type == "journey/start") {
		e.je = newJourneyEnricher()
	}

	return e.je.enrich(event, se, fields)
}

// journeyEnricher holds state across received SynthEvents retaining fields
// where relevant to properly enrich *beat.Event instances.
type journeyEnricher struct {
	journeyComplete bool
	journey         *Journey
	checkGroup      string
	errorCount      int
	firstError      error
	stepCount       int
	// The first URL we visit is the URL for this journey, which is set on the summary event.
	// We store the URL fields here for use on the summary event.
	urlFields common.MapStr
	start     time.Time
	end       time.Time
}

func newJourneyEnricher() *journeyEnricher {
	return &journeyEnricher{
		checkGroup: makeUuid(),
	}
}

func makeUuid() string {
	u, err := uuid.NewV1()
	if err != nil {
		panic("Cannot generate v1 UUID, this should never happen!")
	}
	return u.String()
}

func (je *journeyEnricher) enrich(event *beat.Event, se *SynthEvent, fields StdSuiteFields) error {
	if se == nil {
		return nil
	}

	if !se.Timestamp().IsZero() {
		event.Timestamp = se.Timestamp()
		// Record start and end so we can calculate journey duration accurately later
		switch se.Type {
		case "journey/start":
			je.firstError = nil
			je.checkGroup = makeUuid()
			je.journey = se.Journey
			je.start = event.Timestamp
		case "journey/end", "cmd/status":
			je.end = event.Timestamp
		}
	} else {
		event.Timestamp = time.Now()
	}

	// Id and name differs for inline and suite monitors
	// - We use the monitor id and name for inline journeys ignoring the
	//   autogenerated `inline`journey id and name.
	// - Monitor id/name is concatenated with the journey id/name for
	// 	 suite monitors
	id := fields.Id
	name := fields.Name
	if !fields.IsInline && je.journey != nil {
		id = fmt.Sprintf("%s-%s", id, je.journey.Id)
		name = fmt.Sprintf("%s - %s", name, je.journey.Name)
	}
	eventext.MergeEventFields(event, common.MapStr{
		"event": common.MapStr{
			"type": se.Type,
		},
		"monitor": common.MapStr{
			"check_group": je.checkGroup,
			"id":          id,
			"name":        name,
			"type":        fields.Type,
		},
	})

	// Write suite level fields for suite monitors
	if !fields.IsInline {
		eventext.MergeEventFields(event, common.MapStr{
			"suite": common.MapStr{
				"id":   fields.Id,
				"name": fields.Name,
			},
		})
	}
	return je.enrichSynthEvent(event, se)
}

func (je *journeyEnricher) enrichSynthEvent(event *beat.Event, se *SynthEvent) error {
	var jobErr error
	if se.Error != nil {
		jobErr = stepError(se.Error)
		je.errorCount++
		if je.firstError == nil {
			je.firstError = jobErr
		}
	}

	switch se.Type {
	case "cmd/status":
		// If a command failed _after_ the journey was complete, as it happens
		// when an `afterAll` hook fails, for example, we don't wan't to include
		// a summary in the cmd/status event.
		if !je.journeyComplete {
			return je.createSummary(event)
		}
	case "journey/end":
		je.journeyComplete = true
		return je.createSummary(event)
	case "step/end":
		je.stepCount++
	case "step/screenshot":
		fallthrough
	case "step/screenshot_ref":
		fallthrough
	case "screenshot/block":
		add_data_stream.SetEventDataset(event, "browser.screenshot")
	case "journey/network_info":
		add_data_stream.SetEventDataset(event, "browser.network")
	}

	if se.Id != "" {
		event.SetID(se.Id)
		// This is only relevant for screenshots, which have a specific ID
		// In that case we always want to issue an update op
		event.Meta.Put(events.FieldMetaOpType, events.OpTypeCreate)
	}

	eventext.MergeEventFields(event, se.ToMap())

	if je.urlFields == nil {
		if urlFields, err := event.GetValue("url"); err == nil {
			if ufMap, ok := urlFields.(common.MapStr); ok {
				je.urlFields = ufMap
			}
		}
	}
	return jobErr
}

func (je *journeyEnricher) createSummary(event *beat.Event) error {
	var up, down int
	if je.errorCount > 0 {
		up = 0
		down = 1
	} else {
		up = 1
		down = 0
	}

	// Incase of syntax errors or incorrect runner options, the Synthetics
	// runner would exit immediately with exitCode 1 and we do not set the duration
	// to inform the journey never ran
	if !je.start.IsZero() {
		eventext.MergeEventFields(event, common.MapStr{
			"monitor": common.MapStr{
				"duration": common.MapStr{
					"us": int64(je.end.Sub(je.start) / time.Microsecond),
				},
			},
		})
	}
	eventext.MergeEventFields(event, common.MapStr{
		"url": je.urlFields,
		"event": common.MapStr{
			"type": "heartbeat/summary",
		},
		"synthetics": common.MapStr{
			"type":    "heartbeat/summary",
			"journey": je.journey,
		},
		"summary": common.MapStr{
			"up":   up,
			"down": down,
		},
	})

	if je.journeyComplete {
		return je.firstError
	}

	return fmt.Errorf("journey did not finish executing, %d steps ran", je.stepCount)
}

func stepError(e *SynthError) error {
	return fmt.Errorf("error executing step: %s", e.String())
}
