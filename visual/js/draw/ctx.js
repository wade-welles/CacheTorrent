let Ctx = function() {
  self = this;

  function tick() {
    self.tickers.forEach(function(ticker) {
      ticker.tick();
    });
  }

  function start() {
    self.starters.forEach(function(starter) {
      starter.start();
    });
  }

  /* Fields. */
  self.svg    = d3.select("svg");
  self.width  = +self.svg.attr("width");
  self.height = +self.svg.attr("height");

  self.center = self.svg
    .append("g")
    .attr("transform", "translate(" +
      self.width / 2 + "," +
      self.height / 2 + ")");

  self.simulation = d3.forceSimulation()
    .force("charge", d3.forceManyBody())
    .force("link", d3.forceLink([]).distance(function (d) {
      return 300 + 200 * Math.random();
    }))
    .alphaTarget(1)
    .on("tick", tick);

  self.tickers  = [];
  self.starters = [];

  /* Interface. */
  self.addTicker = function(ticker) {
    self.tickers.push(ticker);
  };

  self.addStarter = function(starter) {
    self.starters.push(starter);
  };

  self.start = function() {
    start();
  };

  return self
};
