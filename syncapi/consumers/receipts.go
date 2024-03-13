// Copyright 2020 The Matrix.org Foundation C.I.C.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package consumers

import (
	"context"
	"strconv"

	"github.com/nats-io/nats.go"
	log "github.com/sirupsen/logrus"

	"github.com/neilalexander/harmony/internal/gomatrixserverlib/spec"
	"github.com/neilalexander/harmony/setup/config"
	"github.com/neilalexander/harmony/setup/jetstream"
	"github.com/neilalexander/harmony/setup/process"
	"github.com/neilalexander/harmony/syncapi/notifier"
	"github.com/neilalexander/harmony/syncapi/storage"
	"github.com/neilalexander/harmony/syncapi/streams"
	"github.com/neilalexander/harmony/syncapi/types"
)

// OutputReceiptEventConsumer consumes events that originated in the EDU server.
type OutputReceiptEventConsumer struct {
	ctx       context.Context
	jetstream nats.JetStreamContext
	durable   string
	topic     string
	db        storage.Database
	stream    streams.StreamProvider
	notifier  *notifier.Notifier
}

// NewOutputReceiptEventConsumer creates a new OutputReceiptEventConsumer.
// Call Start() to begin consuming from the EDU server.
func NewOutputReceiptEventConsumer(
	process *process.ProcessContext,
	cfg *config.SyncAPI,
	js nats.JetStreamContext,
	store storage.Database,
	notifier *notifier.Notifier,
	stream streams.StreamProvider,
) *OutputReceiptEventConsumer {
	return &OutputReceiptEventConsumer{
		ctx:       process.Context(),
		jetstream: js,
		topic:     cfg.Matrix.JetStream.Prefixed(jetstream.OutputReceiptEvent),
		durable:   cfg.Matrix.JetStream.Durable("SyncAPIReceiptConsumer"),
		db:        store,
		notifier:  notifier,
		stream:    stream,
	}
}

// Start consuming receipts events.
func (s *OutputReceiptEventConsumer) Start() error {
	return jetstream.JetStreamConsumer(
		s.ctx, s.jetstream, s.topic, s.durable, 1,
		s.onMessage, nats.DeliverAll(), nats.ManualAck(),
	)
}

func (s *OutputReceiptEventConsumer) onMessage(ctx context.Context, msgs []*nats.Msg) bool {
	msg := msgs[0] // Guaranteed to exist if onMessage is called
	output := types.OutputReceiptEvent{
		UserID:  msg.Header.Get(jetstream.UserID),
		RoomID:  msg.Header.Get(jetstream.RoomID),
		EventID: msg.Header.Get(jetstream.EventID),
		Type:    msg.Header.Get("type"),
	}

	timestamp, err := strconv.ParseUint(msg.Header.Get("timestamp"), 10, 64)
	if err != nil {
		// If the message was invalid, log it and move on to the next message in the stream
		log.WithError(err).Errorf("output log: message parse failure")
		return true
	}

	output.Timestamp = spec.Timestamp(timestamp)

	streamPos, err := s.db.StoreReceipt(
		s.ctx,
		output.RoomID,
		output.Type,
		output.UserID,
		output.EventID,
		output.Timestamp,
	)
	if err != nil {
		return true
	}

	s.stream.Advance(streamPos)
	s.notifier.OnNewReceipt(output.RoomID, types.StreamingToken{ReceiptPosition: streamPos})

	return true
}
