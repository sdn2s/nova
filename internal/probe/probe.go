package probe

import (
	"context"
	"sort"
	"sync"
)

type Starter func(ctx context.Context, name string, args []string) (stop func(), err error)

type Candidate struct {
	Name string
	Args []string
}

type Score struct {
	Name    string
	OK      int
	Fail    int
	SSL     int
	Blocked int
	Unsup   int
	Err     string
	Results []Result
}

func (s Score) Total() int { return s.OK + s.Fail + s.SSL + s.Blocked }

type Prober struct {
	Checker  Checker
	Targets  []Target
	Freeze   []FreezeHost
	Parallel int
	Start    Starter
}

func (p *Prober) Baseline(ctx context.Context) Score {
	return p.checks(ctx, "(no bypass)")
}

func (p *Prober) Strategy(ctx context.Context, c Candidate) Score {
	stop, err := p.Start(ctx, c.Name, c.Args)
	if err != nil {
		return Score{Name: c.Name, Err: err.Error()}
	}
	defer stop()
	return p.checks(ctx, c.Name)
}

func (p *Prober) checks(ctx context.Context, name string) Score {
	type job func() Result
	var jobs []job
	for _, t := range p.Targets {
		for _, test := range Tests {
			jobs = append(jobs, func() Result { return p.Checker.URL(ctx, t.Name, t.URL, test) })
		}
	}
	for _, h := range p.Freeze {
		jobs = append(jobs, func() Result { return p.Checker.Freeze(ctx, h.Provider, h.Host) })
	}

	results := make([]Result, len(jobs))
	sem := make(chan struct{}, max(1, p.Parallel))
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			results[i] = j()
		}()
	}
	wg.Wait()

	s := Score{Name: name, Results: results}
	for _, r := range results {
		switch r.Kind {
		case KindOK:
			s.OK++
		case KindSSL:
			s.SSL++
		case KindBlocked:
			s.Blocked++
		case KindUnsup:
			s.Unsup++
		default:
			s.Fail++
		}
	}
	return s
}

func Rank(scores []Score) []Score {
	out := append([]Score(nil), scores...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.Err == "") != (b.Err == "") {
			return a.Err == ""
		}
		if a.OK != b.OK {
			return a.OK > b.OK
		}
		return a.Blocked < b.Blocked
	})
	return out
}
