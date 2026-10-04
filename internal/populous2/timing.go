package populous2

// SimulationRate is the nominal PAL VBlank cadence of the native main loop.
// Actual original-machine throughput can be lower when the CPU misses a blank;
// the viewport-size word containing eight is not a simulation-rate divisor.
const SimulationRate = 50
