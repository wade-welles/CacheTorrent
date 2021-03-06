package cache_torrent

import (
  . "github.com/danalex97/Speer/interfaces"
  "github.com/danalex97/nfsTorrent/config"

  "sync"
  "sort"
)

var LeaderPercent config.Const = config.NewConst(config.LeaderPercent)

// The Election component is reponsible with Leader election. It runs as a
// centralized component onto the Tracker. The election algorithm runs as
// follows. Each autonomous system represents an Election Camera. When a
// lower bound of 'limit' nodes have joined an entire system, all
// camera election run for the whole system. Nodes joining later do not
// participate in the election. When a camera was elected, the nodes in the
// camera are notfied via a 'Leaders' message.
//
// The nodes are elected leaders by the biggest upload capacity criteria. In
// case of equallity the leaders are chosen by arrival time. Most methods are
// made public so that the MultiTorrent extension can use CacheTorrent
// elections.
type Election struct {
  sync.Mutex

  limit       int
  nodes       int

  camera      map[string][]string
  candidates  map[string][]Candidate

  elected     map[string][]string
  transport   Transport
}

func NewElection(limit int, transport Transport) *Election {
  return &Election{
    camera     : make(map[string][]string),
    candidates : make(map[string][]Candidate),
    elected    : make(map[string][]string),
    limit      : limit,
    nodes      : 0,
    transport  : transport,
  }
}

func (e *Election) Run() {
}

func (e *Election) Recv(m interface {}) {
  switch candidate := m.(type) {
  case Candidate:
    e.RegisterCandidate(candidate)
  }
}

func (e *Election) GetElected() []string {
  e.Lock()
  defer e.Unlock()

  allElected := []string{}
  for _, elected := range e.elected {
    allElected = append(allElected, elected...)
  }
  return allElected
}

func (e *Election) RemoveCandidate(toRemove string) {
  e.Lock()
  defer e.Unlock()

  as := getAS(toRemove)

  candidates    := e.candidates[as]
  newCandidates := []Candidate{}
  for _, candidate := range candidates {
    if candidate.Id != toRemove {
      newCandidates = append(newCandidates, candidate)
    }
  }
  e.candidates[as] = newCandidates
}

// A candidate gets registered when a 'Candidate' message arrives. The
// candidate messages are sent by all Peers directly to the Tracker.
func (e *Election) RegisterCandidate(candidate Candidate) {
  e.Lock()
  defer e.Unlock()

  e.nodes++

  /* Add candidate to candidate list. */
  as := getAS(candidate.Id)
  if _, ok := e.candidates[as]; !ok {
    e.candidates[as] = []Candidate{}
  }
  e.candidates[as] = append(e.candidates[as], candidate)

  // When we reach the node limit, we run the full elections.
  if e.nodes == e.limit {
    e.Unlock()
    e.RunElection()
    e.Lock()
  }

  // For ulterior joins, we only respond with the Leader messages.
  if e.nodes > e.limit {
    elected, ok := e.elected[as]
    if !ok {
      // If there are no leaders, we designate the requester as a leader.
      // i.e. the node will, thus, follow the original BitTorrent protocol
      elected = []string{candidate.Id}
    }
    e.transport.ControlSend(candidate.Id, Leaders{elected})
  }
}

// The NewJoin message updates the Cameras when a new node Joins the system.
func (e *Election) NewJoin(id string) {
  e.Lock()
  defer e.Unlock()

  /* Add id to camera. */
  as := getAS(id)
  if _, ok := e.camera[as]; !ok {
    e.camera[as] = []string{}
  }
  e.camera[as] = append(e.camera[as], id)
}

// Run elections for all autonomous systems. RunElection is called when the
// node limit is reached. This limit is checked when Join messages arrive.
func (e *Election) RunElection() {
  e.Lock()
  defer e.Unlock()

  for as, _ := range e.camera {
    e.Unlock()
    e.elected[as] = e.Elect(as)
    e.Lock()
  }

  // Send Leader messages
  for as, camera := range e.camera {
    elected := e.elected[as]
    for _, node := range camera {
      e.transport.ControlSend(node, Leaders{elected})
    }
  }
}

type byId []Candidate

func (a byId) Len() int           { return len(a) }
func (a byId) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byId) Less(i, j int) bool {
  if a[i].Up != a[j].Up {
    return a[i].Up > a[j].Up
  }
  if a[i].Down != a[j].Down {
    return a[i].Down > a[j].Down
  }
  return a[i].Id < a[j].Id
}

// Run leader election for a specific autonomous system. Candidates are elected
// the the biggest upload capacity criteria. In case of equallity, the nodes'
// download capacity is compared and, finally, the provided ID.
func (e *Election) Elect(as string) []string {
  e.Lock()
  defer e.Unlock()

  candidates, ok := e.candidates[as]
  if !ok {
    // If there are no candidates, we designate all nodes as leaders,
    // i.e. each node will be able to communicate with the exterior
    return e.camera[as]
  }

  // Sort the candidates by a criteria
  sort.Sort(byId(candidates))

  leaders    := []string{}
  maxLeaders := len(candidates) * LeaderPercent.Int() / 100
  if maxLeaders == 0 {
    maxLeaders = 1
  }
  if len(candidates) < maxLeaders {
    maxLeaders = len(candidates)
  }
  for i := 0; i < maxLeaders; i++ {
    leaders = append(leaders, candidates[i].Id)
  }

  return leaders
}
