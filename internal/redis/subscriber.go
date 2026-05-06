package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	rds "github.com/redis/go-redis/v9"
)

// Subscriber подписывается на каналы документов и отдаёт дельты в общий канал.
//
// Дизайн: один процесс держит один Subscriber и подписывается на все документы,
// активные на этом узле. Subscribe/Unsubscribe идемпотентны.
type Subscriber struct {
	client rds.UniversalClient

	mu     sync.Mutex
	pubsub *rds.PubSub
	subs   map[string]struct{} // подписанные documentID

	out    chan Delta
	stop   chan struct{}
	once   sync.Once
	closed bool

	bufSize int
}

// NewSubscriber создаёт подписчика. bufSize <= 0 → дефолт 256.
//
// Run() должен быть запущен в отдельной горутине после первого Subscribe,
// либо подписчик стартует автоматически при первом Subscribe.
func NewSubscriber(client rds.UniversalClient, bufSize int) *Subscriber {
	if bufSize <= 0 {
		bufSize = 256
	}
	return &Subscriber{
		client:  client,
		subs:    make(map[string]struct{}),
		out:     make(chan Delta, bufSize),
		stop:    make(chan struct{}),
		bufSize: bufSize,
	}
}

// Messages возвращает канал входящих дельт.
func (s *Subscriber) Messages() <-chan Delta { return s.out }

// Subscribe добавляет подписку на documentID. Идемпотентно.
func (s *Subscriber) Subscribe(ctx context.Context, documentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("redis subscriber: closed")
	}
	if _, ok := s.subs[documentID]; ok {
		return nil
	}
	if s.pubsub == nil {
		s.pubsub = s.client.Subscribe(ctx, channelFor(documentID))
		go s.run()
	} else {
		if err := s.pubsub.Subscribe(ctx, channelFor(documentID)); err != nil {
			return fmt.Errorf("redis subscribe: %w", err)
		}
	}
	s.subs[documentID] = struct{}{}
	return nil
}

// Unsubscribe убирает подписку. Идемпотентно.
func (s *Subscriber) Unsubscribe(ctx context.Context, documentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pubsub == nil {
		return nil
	}
	if _, ok := s.subs[documentID]; !ok {
		return nil
	}
	if err := s.pubsub.Unsubscribe(ctx, channelFor(documentID)); err != nil {
		return fmt.Errorf("redis unsubscribe: %w", err)
	}
	delete(s.subs, documentID)
	return nil
}

// Close останавливает подписчика и закрывает выходной канал. Идемпотентен.
func (s *Subscriber) Close() error {
	var err error
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		ps := s.pubsub
		s.mu.Unlock()
		close(s.stop)
		if ps != nil {
			err = ps.Close()
		}
	})
	return err
}

// run — диспетчер сообщений pubsub→out.
// Завершается при Close или ошибке pubsub.
func (s *Subscriber) run() {
	s.mu.Lock()
	ch := s.pubsub.Channel(rds.WithChannelSize(s.bufSize))
	s.mu.Unlock()

	defer close(s.out)
	for {
		select {
		case <-s.stop:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var delta Delta
			if err := json.Unmarshal([]byte(msg.Payload), &delta); err != nil {
				continue
			}
			select {
			case s.out <- delta:
			case <-s.stop:
				return
			}
		}
	}
}
