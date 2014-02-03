package metrics

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"time"
)

// GraphiteConfig provides a container with configuration parameters for
// the Graphite exporter
type GraphiteConfig struct {
	Addr          *net.TCPAddr  // Network address to connect to
	Registry      Registry      // Registry to be exported
	FlushInterval time.Duration // Flush interval
	DurationUnit  time.Duration // Time conversion unit for durations
	Prefix        string        // Prefix to be prepended to metric names
}

// Graphite is a blocking exporter function which reports metrics in r
// to a graphite server located at addr, flushing them every d duration
// and prepending metric names with prefix.
func Graphite(r Registry, d time.Duration, prefix string, addr *net.TCPAddr) {
	GraphiteWithConfig(GraphiteConfig{
		Addr:          addr,
		Registry:      r,
		FlushInterval: d,
		DurationUnit:  time.Nanosecond,
		Prefix:        prefix,
	})
}

// GraphiteWithConfig is a blocking exporter function just like Graphite,
// but it takes a GraphiteConfig instead.
func GraphiteWithConfig(c GraphiteConfig) {
	for _ = range time.Tick(c.FlushInterval) {
		if err := graphite(&c); nil != err {
			log.Println(err)
		}
	}
}

func graphite(c *GraphiteConfig) error {
	now := time.Now().Unix()
	du := float64(c.DurationUnit)
	conn, err := net.DialTCP("tcp", nil, c.Addr)
	if nil != err {
		return err
	}
	defer conn.Close()
	w := bufio.NewWriter(conn)
	c.Registry.Each(func(name string, i interface{}) {
		switch metric := i.(type) {
		case Counter:
			fmt.Fprintf(w, "%s.%s.count %d %d\n", c.Prefix, name, metric.Count(), now)
		case Gauge:
			fmt.Fprintf(w, "%s.%s.value %d %d\n", c.Prefix, name, metric.Value(), now)
		case Histogram:
			h := metric.Snapshot()
			ps := h.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
			fmt.Fprintf(w, "%s.%s.count %d %d\n", c.Prefix, name, h.Count(), now)
			fmt.Fprintf(w, "%s.%s.min %d %d\n", c.Prefix, name, h.Min(), now)
			fmt.Fprintf(w, "%s.%s.max %d %d\n", c.Prefix, name, h.Max(), now)
			fmt.Fprintf(w, "%s.%s.mean %.2f %d\n", c.Prefix, name, h.Mean(), now)
			fmt.Fprintf(w, "%s.%s.std-dev %.2f %d\n", c.Prefix, name, h.StdDev(), now)
			fmt.Fprintf(w, "%s.%s.50-percentile %.2f %d\n", c.Prefix, name, ps[0], now)
			fmt.Fprintf(w, "%s.%s.75-percentile %.2f %d\n", c.Prefix, name, ps[1], now)
			fmt.Fprintf(w, "%s.%s.95-percentile %.2f %d\n", c.Prefix, name, ps[2], now)
			fmt.Fprintf(w, "%s.%s.99-percentile %.2f %d\n", c.Prefix, name, ps[3], now)
			fmt.Fprintf(w, "%s.%s.999-percentile %.2f %d\n", c.Prefix, name, ps[4], now)
		case Meter:
			m := metric.Snapshot()
			fmt.Fprintf(w, "%s.%s.count %d %d\n", c.Prefix, name, m.Count(), now)
			fmt.Fprintf(w, "%s.%s.one-minute %.2f %d\n", c.Prefix, name, m.Rate1(), now)
			fmt.Fprintf(w, "%s.%s.five-minute %.2f %d\n", c.Prefix, name, m.Rate5(), now)
			fmt.Fprintf(w, "%s.%s.fifteen-minute %.2f %d\n", c.Prefix, name, m.Rate15(), now)
			fmt.Fprintf(w, "%s.%s.mean %.2f %d\n", c.Prefix, name, m.RateMean(), now)
		case Timer:
			t := metric.Snapshot()
			ps := t.Percentiles([]float64{0.5, 0.75, 0.95, 0.99, 0.999})
			fmt.Fprintf(w, "%s.%s.count %d %d\n", c.Prefix, name, t.Count(), now)
			fmt.Fprintf(w, "%s.%s.min %d %d\n", c.Prefix, name, int64(du)*t.Min(), now)
			fmt.Fprintf(w, "%s.%s.max %d %d\n", c.Prefix, name, int64(du)*t.Max(), now)
			fmt.Fprintf(w, "%s.%s.mean %.2f %d\n", c.Prefix, name, du*t.Mean(), now)
			fmt.Fprintf(w, "%s.%s.std-dev %.2f %d\n", c.Prefix, name, du*t.StdDev(), now)
			fmt.Fprintf(w, "%s.%s.50-percentile %.2f %d\n", c.Prefix, name, du*ps[0], now)
			fmt.Fprintf(w, "%s.%s.75-percentile %.2f %d\n", c.Prefix, name, du*ps[1], now)
			fmt.Fprintf(w, "%s.%s.95-percentile %.2f %d\n", c.Prefix, name, du*ps[2], now)
			fmt.Fprintf(w, "%s.%s.99-percentile %.2f %d\n", c.Prefix, name, du*ps[3], now)
			fmt.Fprintf(w, "%s.%s.999-percentile %.2f %d\n", c.Prefix, name, du*ps[4], now)
			fmt.Fprintf(w, "%s.%s.one-minute %.2f %d\n", c.Prefix, name, t.Rate1(), now)
			fmt.Fprintf(w, "%s.%s.five-minute %.2f %d\n", c.Prefix, name, t.Rate5(), now)
			fmt.Fprintf(w, "%s.%s.fifteen-minute %.2f %d\n", c.Prefix, name, t.Rate15(), now)
			fmt.Fprintf(w, "%s.%s.mean-rate %.2f %d\n", c.Prefix, name, t.RateMean(), now)
		}
		w.Flush()
	})
	return nil
}
