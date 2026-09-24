package valkey

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-tangra/go-tangra/v4/authz"
)

// Source loads the policy document from Valkey and reloads on pub/sub
// notifications (and on a slow poll as a safety net). Invalid documents are
// never delivered; the last-known-good policy stays active.
type Source struct {
	kv           KV
	td           string
	pollInterval time.Duration
	mu           sync.Mutex
	cur          *authz.Policy
	version      string
	subs         []chan *authz.Policy
	errs         chan error
	stop         chan struct{}
	once         sync.Once
	closed       bool
}

var _ authz.Source = (*Source)(nil)

// NewSource loads the document for trustDomain immediately.
func NewSource(ctx context.Context, kv KV, trustDomain string, pollInterval time.Duration) (*Source, error) {
	if pollInterval <= 0 {
		pollInterval = 30 * time.Second
	}
	s := &Source{kv: kv, td: trustDomain, pollInterval: pollInterval, errs: make(chan error, 16), stop: make(chan struct{})}
	pol, ver, err := s.fetch(ctx)
	if err != nil {
		return nil, err
	}
	s.cur, s.version = pol, ver
	return s, nil
}

func (s *Source) fetch(ctx context.Context) (*authz.Policy, string, error) {
	doc, ok, err := s.kv.Get(ctx, docKey(s.td))
	if err != nil {
		return nil, "", fmt.Errorf("policy-valkey: %w", err)
	}
	if !ok {
		return nil, "", errors.New("policy-valkey: policy document not found")
	}
	ver, _, err := s.kv.Get(ctx, versionKey(s.td))
	if err != nil {
		return nil, "", fmt.Errorf("policy-valkey: %w", err)
	}
	pol, err := authz.Load(strings.NewReader(doc))
	if err != nil {
		return nil, "", err
	}
	if ver != "" && ver != pol.Version {
		return nil, "", fmt.Errorf("policy-valkey: version key %q does not match document version %q", ver, pol.Version)
	}
	pol.Source = "valkey:" + docKey(s.td)
	return pol, pol.Version, nil
}

// Load implements authz.Source.
func (s *Source) Load(context.Context) (*authz.Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur == nil {
		return nil, errors.New("policy-valkey: no policy loaded")
	}
	return s.cur, nil
}

// Watch implements authz.Source.
func (s *Source) Watch(ctx context.Context) (<-chan *authz.Policy, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("policy-valkey: source closed")
	}
	ch := make(chan *authz.Policy, 4)
	s.subs = append(s.subs, ch)
	s.mu.Unlock()
	s.once.Do(func() {
		go s.subscribe()
		go s.poll()
	})
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

// Errors reports failed reloads.
func (s *Source) Errors() <-chan error { return s.errs }

func (s *Source) subscribe() {
	for {
		ctx, cancel := context.WithCancel(context.Background())
		go func() { <-s.stop; cancel() }()
		err := s.kv.Subscribe(ctx, ChangedChannel, func(string) { s.reload() })
		cancel()
		select {
		case <-s.stop:
			return
		default:
		}
		if err != nil {
			s.report(fmt.Errorf("policy-valkey: subscribe: %w", err))
		}
		time.Sleep(time.Second)
	}
}

func (s *Source) poll() {
	t := time.NewTicker(s.pollInterval)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.reload()
		}
	}
}

func (s *Source) reload() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pol, ver, err := s.fetch(ctx)
	if err != nil {
		s.report(err)
		return
	}
	s.mu.Lock()
	if ver == s.version {
		s.mu.Unlock()
		return
	}
	s.cur, s.version = pol, ver
	subs := append([]chan *authz.Policy(nil), s.subs...)
	s.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- pol:
		default:
		}
	}
}

func (s *Source) report(err error) {
	select {
	case s.errs <- err:
	default:
	}
}

// Close stops watching.
func (s *Source) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.stop)
	for _, ch := range s.subs {
		close(ch)
	}
	s.subs = nil
	return nil
}
