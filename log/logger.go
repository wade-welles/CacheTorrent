package log

import (
  "runtime"
  "encoding/json"
  "sync"
  "fmt"
  "os"
)

var Log *Logger = NewLogger()

var print   = fmt.Print
var printf  = fmt.Printf
var println = fmt.Println

const (
  GetRedundancy = iota
  GetTime       = iota
  GetTimeLeader = iota
  GetTraffic    = iota
  GetTimeCDF    = iota
  GetLeaderCDF  = iota
  GetLogged     = iota
  Stop          = iota
)

const maxTransfers int = 100000
const maxCompletes int = 1000

type piece struct {
  as  string
  idx int
}

type Logger struct {
  verbose    bool

  isLeader   map[string]bool
  redundancy map[piece]int
  traffic    map[int]int
  times      map[string]int

  leaders   chan Leader
  transfers chan Transfer
  completes chan Completed
  queries   chan int
  packets   chan Packet

  logged    []Packet    `json:"packets"`
  lock      *sync.Mutex
  logfile   string

  stopped   bool
  wg        *sync.WaitGroup
}

func NewLogger() *Logger {
  logger := &Logger{
    verbose : false,

    isLeader   : make(map[string]bool),
    redundancy : make(map[piece]int),
    traffic    : make(map[int]int),
    times      : make(map[string]int),

    leaders    : make(chan Leader, maxCompletes),
    transfers  : make(chan Transfer, maxTransfers),
    completes  : make(chan Completed, maxCompletes),
    packets    : make(chan Packet, maxTransfers),
    queries    : make(chan int, 1),

    logged     : []Packet{},
    logfile    : "",
    lock       : new(sync.Mutex),

    stopped    : false,
    wg         : new(sync.WaitGroup),
  }

  go logger.run()

  return logger
}

/* Defaults*/
func Wait()                    { Log.Wait() }
func SetVerbose(verbose bool)  { Log.SetVerbose(verbose) }
func SetLogfile(logfile string){ Log.SetLogfile(logfile) }
func HasLogfile() bool         { return Log.HasLogfile() }
func Println(v ...interface{}) { Log.Println(v...) }
func LogPacket(t Packet)       { Log.LogPacket(t) }
func LogLeader(t Leader)       { Log.LogLeader(t) }
func LogCompleted(t Completed) { Log.LogCompleted(t) }
func LogTransfer(t Transfer)   { Log.LogTransfer(t) }
func Query(q int)              { Log.Query(q) }

/* Interface. */
func (l *Logger) Wait() {
  l.wg.Wait()
}

func (l *Logger) SetVerbose(verbose bool) {
  l.verbose = verbose
}

func (l *Logger) SetLogfile(logfile string) {
  l.lock.Lock()
  defer l.lock.Unlock()

  l.logfile = logfile
}

func (l *Logger) HasLogfile() bool {
  l.lock.Lock()
  defer l.lock.Unlock()

  return l.logfile != ""
}

func (l *Logger) Println(v ...interface{}) {
  if l.verbose {
    fmt.Println(v...)
  }
}

func (l *Logger) LogPacket(t Packet) {
  l.packets <- t
}

func (l *Logger) LogLeader(t Leader) {
  l.leaders <- t
}

func (l *Logger) LogCompleted(t Completed) {
  l.completes <- t
}

func (l *Logger) LogTransfer(t Transfer) {
  l.transfers <- t
}

func (l *Logger) Query(q int) {
  l.queries <- q
}

/* Handlers. */
func (l *Logger) handleLeader(le Leader) {
  leader := le.Id
  l.isLeader[leader] = true
}

func (l *Logger) handlePacket(p Packet) {
  l.logged = append(l.logged, p)
}

func (l *Logger) handleTransfer(t Transfer) {
  // Support for MultiTorrents
  t.From = stripId(t.From)
  t.To   = stripId(t.To)

  as := getAS(t.To)
  if as != getAS(t.From) {
    p := piece{
      as  : as,
      idx : t.Index,
    }
    if _, ok := l.redundancy[p]; !ok {
      l.redundancy[p] = 0
    }
    l.redundancy[p] += 1
  }

  idx := t.Index
  if _, ok := l.traffic[idx]; !ok {
    l.traffic[idx] = 0
  }
  l.traffic[idx] += 1
}

func (l *Logger) handleComplete(c Completed) {
  // Support for MultiTorrents
  c.Id = stripId(c.Id)

  l.times[c.Id] = c.Time
}

/* Queries. */
func (l *Logger) getLogged() {
  b, err := json.Marshal(l.logged)

  if err == nil {
    f, err := os.Create(l.logfile)
    if err == nil {
      defer f.Close()
      f.Write(b)

      printf("Logfile %s written.\n", l.logfile)
    }
  }
}

func (l *Logger) getRedundancy() {
  pieces := 0
  times  := 0
  for _, ctr := range l.redundancy {
    pieces += 1
    times  += ctr
  }
  redundancy := float64(times) / float64(pieces)
  println("Redundancy:", redundancy)
}

func (l *Logger) getTraffic() {
  total := 0
  peers := 0
  for _, ctr := range l.traffic {
    total += ctr
    peers += 1
  }
  traffic := float64(total) / float64(peers)
  println("Traffic:", traffic)
}

func (l *Logger) getTime() {
  times := toSlice(l.times)

  println("Average time:", getAverage(times))
  println("50th percentile:", getPercentile(50.0, times))
  println("90th percentile:", getPercentile(90.0, times))
}

func (l *Logger) getLeaderTimes() ([]int, []int) {
  leaderTimes   := []int{}
  followerTimes := []int{}

  for id, time := range l.times {
    if _, ok := l.isLeader[id]; ok {
      leaderTimes = append(leaderTimes, time)
    } else {
      followerTimes = append(followerTimes, time)
    }
  }
  mnLeader   := minSlice(leaderTimes)
  mnFollower := minSlice(followerTimes)
  if mnLeader < mnFollower {
    followerTimes = append(followerTimes, mnLeader)
  } else {
    leaderTimes = append(leaderTimes, mnFollower)
  }

  return leaderTimes, followerTimes
}

func (l *Logger) getTimeLeader() {
  leaderTimes, followerTimes := l.getLeaderTimes()

  println("Leader 50th percentile:", getPercentile(50.0, leaderTimes))
  println("Leader 90th percentile:", getPercentile(90.0, leaderTimes))
  println("Follower 50th percentile:", getPercentile(50.0, followerTimes))
  println("Follower 90th percentile:", getPercentile(90.0, followerTimes))
}

func (l *Logger) getLeaderCDF() {
  leaderTimes, followerTimes := l.getLeaderTimes()

  print("Leader time CDF: [")
  for _, t := range normalize(leaderTimes) {
    print(t, ",")
  }
  println("]")

  print("Follower time CDF: [")
  for _, t := range normalize(followerTimes) {
    print(t, ",")
  }
  println("]")
}

func (l *Logger) getTimeCDF() {
  print("Time CDF: [")
  for _, t := range normalize(toSlice(l.times)) {
    print(t, ",")
  }
  println("]")
}

/* Runner. */
func (l *Logger) run() {
  l.wg.Add(1)

  for {
    select {
    case t := <-l.leaders:
      l.handleLeader(t)
      continue
    default:
    }

    select {
    case t := <-l.packets:
      l.handlePacket(t)
      continue
    default:
    }

    select {
    case t := <-l.transfers:
      l.handleTransfer(t)
      continue
    default:
    }

    select {
    case c := <-l.completes:
      l.handleComplete(c)
      continue
    default:
    }

    select {
    case q := <-l.queries:
      switch q {
      case GetTimeLeader:
        l.getTimeLeader()
      case GetRedundancy:
        l.getRedundancy()
      case GetTime:
        l.getTime()
      case GetTraffic:
        l.getTraffic()
      case GetTimeCDF:
        l.getTimeCDF()
      case GetLeaderCDF:
        l.getLeaderCDF()
      case GetLogged:
        l.getLogged()
      case Stop:
        l.stopped = true
      }
      continue
    default:
    }

    // All channels are drained
    if l.stopped {
      break
    }
    runtime.Gosched()
  }

  l.wg.Done()
}
