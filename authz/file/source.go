package file

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-freya/freya/authz"
)

// Source loads a policy file and polls it for changes. It never yields an
// invalid policy: failed reloads are reported on Errors() and the last-known-
// good policy stays active.
type Source struct {
	path   string
	poll   time.Duration
	mu     sync.Mutex
	cur    *authz.Policy
	hash   [32]byte
	subs   []chan *authz.Policy
	errs   chan error
	stop   chan struct{}
	done   chan struct{}
	once   sync.Once
	closed bool
}

var _ authz.Source = (*Source)(nil)

// Option configures the source.
type Option func(*Source)

// WithPollInterval sets the change-detection interval (default 2s).
func WithPollInterval(d time.Duration) Option { return func(s *Source) { s.poll = d } }

// New loads path immediately and fails if it is missing or invalid.
func New(path string, opts ...Option) (*Source, error) {
	s := &Source{path: path, poll: 2 * time.Second, errs: make(chan error, 16), stop: make(chan struct{}), done: make(chan struct{})}
	for _, o := range opts {
		o(s)
	}
	pol, hash, err := s.read()
	if err != nil {
		return nil, err
	}
	s.cur, s.hash = pol, hash
	return s, nil
}

func (s *Source) read() (*authz.Policy, [32]byte, error) {
	raw, err := os.ReadFile(s.path) // #nosec G304 -- operator-supplied policy path
	if err != nil {
		return nil, [32]byte{}, fmt.Errorf("authz/file: %w", err)
	}
	pol, err := authz.Load(bytes.NewReader(raw))
	if err != nil {
		return nil, [32]byte{}, fmt.Errorf("authz/file: %s: %w", s.path, err)
	}
	pol.Source = "file:" + s.path
	return pol, sha256.Sum256(raw), nil
}

// Load implements authz.Source.
func (s *Source) Load(context.Context) (*authz.Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur == nil {
		return nil, errors.New("authz/file: no policy loaded")
	}
	return s.cur, nil
}

// Watch implements authz.Source.
func (s *Source) Watch(ctx context.Context) (<-chan *authz.Policy, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("authz/file: source closed")
	}
	ch := make(chan *authz.Policy, 4)
	s.subs = append(s.subs, ch)
	s.mu.Unlock()
	s.once.Do(func() { go s.loop() })
	go func() {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			for i, c := range s.subs {
				if c == ch {
					s.subs = append(s.subs[:i], s.subs[i+1:]...)
					close(ch)
					break
				}
			}
			s.mu.Unlock()
		case <-s.stop:
		}
	}()
	return ch, nil
}

// Errors reports failed reloads (last-known-good remains active).
func (s *Source) Errors() <-chan error { return s.errs }

func (s *Source) loop() {
	defer close(s.done)
	t := time.NewTicker(s.poll)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			pol, hash, err := s.read()
			if err != nil {
				select {
				case s.errs <- err:
				default:
				}
				continue
			}
			s.mu.Lock()
			if hash == s.hash {
				s.mu.Unlock()
				continue
			}
			s.cur, s.hash = pol, hash
			subs := append([]chan *authz.Policy(nil), s.subs...)
			s.mu.Unlock()
			for _, ch := range subs {
				select {
				case ch <- pol:
				default:
				}
			}
		}
	}
}

// Close stops polling and closes watch channels.
func (s *Source) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.stop)
	for _, ch := range s.subs {
		close(ch)
	}
	s.subs = nil
	s.mu.Unlock()
	return nil
}
