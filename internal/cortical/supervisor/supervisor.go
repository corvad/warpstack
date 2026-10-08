package supervisor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

type Spec struct {
	Subsystem Subsystem
	Options   Options
}

type Options struct {
	MinBackoff        time.Duration
	MaxBackoff        time.Duration
	StableBackoff     time.Duration
	BackoffMultiplier float64
}

type Subsystem interface {
	Name() string
	Init(ctx context.Context) error
	Run(ctx context.Context) error
}

type Supervisor struct {
	specs []Spec
	wg    sync.WaitGroup
}

func New(specs []Spec) (*Supervisor, error) {
	for i := range specs {
		if specs[i].Subsystem == nil {
			return nil, fmt.Errorf("subsystem %d is nil", i)
		}
		if specs[i].Options.MinBackoff <= 0 {
			return nil, fmt.Errorf("invalid MinBackoff for subsystem %s: %v", specs[i].Subsystem.Name(), specs[i].Options.MinBackoff)
		}
		if specs[i].Options.MaxBackoff <= 0 {
			return nil, fmt.Errorf("invalid MaxBackoff for subsystem %s: %v", specs[i].Subsystem.Name(), specs[i].Options.MaxBackoff)
		}
		if specs[i].Options.MinBackoff > specs[i].Options.MaxBackoff {
			return nil, fmt.Errorf("MinBackoff cannot be greater than MaxBackoff for subsystem %s", specs[i].Subsystem.Name())
		}
		if specs[i].Options.StableBackoff <= 0 {
			return nil, fmt.Errorf("invalid StableBackoff for subsystem %s: %v", specs[i].Subsystem.Name(), specs[i].Options.StableBackoff)
		}
		if specs[i].Options.BackoffMultiplier < 1 {
			return nil, fmt.Errorf("invalid BackoffMultiplier for subsystem %s: %v", specs[i].Subsystem.Name(), specs[i].Options.BackoffMultiplier)
		}
	}
	return &Supervisor{
		specs: specs,
		wg:    sync.WaitGroup{},
	}, nil
}

func (s *Supervisor) Init(ctx context.Context) error {
	for _, subsystem := range s.specs {
		if err := subsystem.Subsystem.Init(ctx); err != nil {
			return fmt.Errorf("init %s: %w", subsystem.Subsystem.Name(), err)
		}
	}
	return nil
}

func (s *Supervisor) Run(ctx context.Context) error {
	for _, subsystem := range s.specs {
		s.wg.Add(1)
		go func(subsystem Spec) {
			defer s.wg.Done()
			s.supervise(ctx, subsystem)
		}(subsystem)
	}
	s.wg.Wait()
	return nil
}

func run(ctx context.Context, subsystem Subsystem) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("subsystem panic: %v", r)
		}
	}()
	return subsystem.Run(ctx)
}

func (s *Supervisor) supervise(ctx context.Context, spec Spec) {
	backoff := spec.Options.MinBackoff
	for {
		start := time.Now()
		err := run(ctx, spec.Subsystem)
		if ctx.Err() != nil {
			log.Printf("[supervisor] subsystem %s quit gracefully", spec.Subsystem.Name())
			return
		}
		if err == nil {
			err = errors.New("subsystem exited unexpectedly")
		}
		if err != nil {
			backoff = time.Duration(float64(backoff) * spec.Options.BackoffMultiplier)
			if backoff > spec.Options.MaxBackoff {
				backoff = spec.Options.MaxBackoff
			}
			if time.Since(start) > spec.Options.StableBackoff {
				backoff = spec.Options.MinBackoff
			}
			log.Printf("[supervisor] subsystem %s failed: %v", spec.Subsystem.Name(), err)
			log.Printf("[supervisor] subsystem %s will restart after backoff of %v", spec.Subsystem.Name(), backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
		}
	}
}
