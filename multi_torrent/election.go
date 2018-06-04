package multi_torrent

import (
  . "github.com/danalex97/Speer/interfaces"

  "github.com/danalex97/nfsTorrent/cache_torrent"
  "strconv"
)

type MultiElection struct {
  elections []*cache_torrent.Election
}

func NewMultiElection(elections int, limit int, transport Transport) *MultiElection {
  e := &MultiElection{
    elections : []*cache_torrent.Election{},
  }

  for i := 0; i < elections; i++ {
    e.elections = append(e.elections, cache_torrent.NewElection(limit, transport))
  }
  return e
}

func (e *MultiElection) Run() {
}

func (e *MultiElection) NewJoin(id string) {
  for i, election := range e.elections {
    election.NewJoin(FullId(id, strconv.Itoa(i)))
  }
}

func (e *MultiElection) Recv(m interface {}) {
  switch candidate := m.(type) {
  case cache_torrent.Candidate:
    e.RegisterCandidate(candidate)
  }
}

func (e *MultiElection) RegisterCandidate(candidate cache_torrent.Candidate) {
  for _, election := range e.elections {
    election.RegisterCandidate(candidate)
  }
}
